package util

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	ociCommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/resource"
)

// ResourceSearchAPI is the subset of OCI's Resource Search client this file depends on --
// mirrors OCISecretsManagerAPI/NewRelicClientAPI's mockable-interface pattern elsewhere in this
// package.
type ResourceSearchAPI interface {
	SearchResources(ctx context.Context, request resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error)
}

var (
	resourceSearchClientMu    sync.Mutex
	cachedResourceSearchAPI   ResourceSearchAPI
	cachedResourceSearchErr   error
	resourceSearchClientCache time.Time
)

// NewResourceSearchClient returns a TTL-cached OCI Resource Search client, authenticated via
// Resource Principal -- the same mechanism this package already uses for its Secrets client
// (secrets_util.go), so this feature needs no new customer credential flow. A cached success is
// kept for getClientTTL (shared with NewNRClient); a cached failure is kept for the much shorter
// getResourceSearchErrorTTL, see resourceSearchCacheTTL. Caches on cachedAt being non-zero rather
// than the client being non-nil, deliberately: createResourceSearchClient's two error paths both
// return a real nil, so a nil-pointer check here would never actually cache a failure and would
// retry on every single call.
func NewResourceSearchClient() (ResourceSearchAPI, error) {
	resourceSearchClientMu.Lock()
	defer resourceSearchClientMu.Unlock()

	if !resourceSearchClientCache.IsZero() && time.Since(resourceSearchClientCache) < resourceSearchCacheTTL(cachedResourceSearchErr) {
		return cachedResourceSearchAPI, cachedResourceSearchErr
	}

	cachedResourceSearchAPI, cachedResourceSearchErr = createResourceSearchClient()
	resourceSearchClientCache = time.Now()

	return cachedResourceSearchAPI, cachedResourceSearchErr
}

// resourceSearchCacheTTL returns how long a cached NewResourceSearchClient result should be
// reused: the normal getClientTTL (shared with NewNRClient) for a cached success, or the much
// shorter getResourceSearchErrorTTL for a cached failure -- so a transient Resource Principal
// auth blip on cold start is retried quickly instead of leaving enrichment dark for the full
// success TTL window.
func resourceSearchCacheTTL(cachedErr error) time.Duration {
	if cachedErr != nil {
		return getResourceSearchErrorTTL()
	}
	return getClientTTL()
}

// getResourceSearchErrorTTL returns the TTL for a cached failed resource search client creation,
// from environment variable or default. Mirrors getClientTTL's env-parsing shape.
func getResourceSearchErrorTTL() time.Duration {
	ttlSeconds := common.DefaultResourceSearchErrorTTL

	if envTTL := os.Getenv(common.ResourceSearchErrorTTL); envTTL != "" {
		if parsedTTL, err := strconv.Atoi(envTTL); err == nil && parsedTTL > 0 {
			ttlSeconds = parsedTTL
		}
	}

	return time.Duration(ttlSeconds) * time.Second
}

func createResourceSearchClient() (ResourceSearchAPI, error) {
	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		log.WithField("error", err).Error("failed to create resource principal configuration provider")
		return nil, fmt.Errorf("failed to create resource principal configuration provider: %w", err)
	}

	client, err := resourcesearch.NewResourceSearchClientWithConfigurationProvider(provider)
	if err != nil {
		log.WithField("error", err).Error("failed to create OCI resource search client")
		return nil, fmt.Errorf("failed to create OCI resource search client: %w", err)
	}

	return &client, nil
}

// nameCacheEntry is one cached OCID -> display name result. name is intentionally allowed to be
// empty: a chunk that came back from Resource Search without a match (or without a DisplayName)
// for a given OCID is just as cacheable as a hit -- otherwise a persistently unresolvable OCID
// (deleted resource, no read policy, etc.) would pay a fresh Resource Search round trip on every
// single invocation for as long as the container stays warm.
type nameCacheEntry struct {
	name       string
	resolvedAt time.Time
}

var (
	nameCacheMu sync.RWMutex
	nameCache   = map[string]nameCacheEntry{}
)

// nameCacheTTL returns how long a cached OCID -> name result is reused, from environment
// variable or default. Mirrors getResourceSearchErrorTTL's env-parsing shape.
func nameCacheTTL() time.Duration {
	ttlSeconds := common.DefaultOCIDNameCacheTTL

	if envTTL := os.Getenv(common.OCIDNameCacheTTL); envTTL != "" {
		if parsedTTL, err := strconv.Atoi(envTTL); err == nil && parsedTTL > 0 {
			ttlSeconds = parsedTTL
		}
	}

	return time.Duration(ttlSeconds) * time.Second
}

// ResourceSearchResolver implements resource.Resolver using a live OCI Resource Search client.
type ResourceSearchResolver struct {
	client ResourceSearchAPI
}

var _ resource.Resolver = (*ResourceSearchResolver)(nil)

// NewResourceSearchResolver wraps client as a resource.Resolver, ready to pass into
// resource.EnrichRecords.
func NewResourceSearchResolver(client ResourceSearchAPI) *ResourceSearchResolver {
	return &ResourceSearchResolver{client: client}
}

// ResolveMany implements resource.Resolver: dedupes ocids defensively (EnrichRecords already
// dedupes via its pending set, but this stays correct standalone), serves whatever it can from
// the cache, chunks whatever's left to common.MaxIdentifiersPerQuery (OCI's verified hard limit
// on an "identifier in (...)" clause), and runs chunks concurrently through a semaphore sized
// common.ResourceSearchWorkerPool. One chunk's failure is logged and doesn't block the others --
// partial results beat none, the same continue-on-error shape ConsumeLogBatches already uses
// elsewhere in this module. The first error encountered (if any) is still returned so the caller
// knows resolution was incomplete.
func (r *ResourceSearchResolver) ResolveMany(ctx context.Context, ocids []string) (map[string]string, error) {
	unique := dedupeOCIDs(ocids)

	results := make(map[string]string)
	toResolve := make([]string, 0, len(unique))

	ttl := nameCacheTTL()
	now := time.Now()
	nameCacheMu.RLock()
	for _, ocid := range unique {
		if entry, ok := nameCache[ocid]; ok && now.Sub(entry.resolvedAt) < ttl {
			if entry.name != "" {
				results[ocid] = entry.name
			}
			continue
		}
		toResolve = append(toResolve, ocid)
	}
	nameCacheMu.RUnlock()

	log.Debugf("resource search: %d of %d distinct OCID(s) served from cache, %d need a live lookup", len(unique)-len(toResolve), len(unique), len(toResolve))

	if len(toResolve) == 0 {
		return results, nil
	}

	var chunks [][]string
	for i := 0; i < len(toResolve); i += common.MaxIdentifiersPerQuery {
		end := i + common.MaxIdentifiersPerQuery
		if end > len(toResolve) {
			end = len(toResolve)
		}
		chunks = append(chunks, toResolve[i:end])
	}

	log.Debugf("resource search: resolving %d distinct OCID(s) across %d chunk(s), worker pool=%d", len(toResolve), len(chunks), common.ResourceSearchWorkerPool)

	var (
		wg        sync.WaitGroup
		sem       = make(chan struct{}, common.ResourceSearchWorkerPool)
		resultsMu sync.Mutex
		errMu     sync.Mutex
		firstErr  error
	)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			items, err := r.searchChunk(ctx, chunk)
			elapsed := time.Since(start)
			if err != nil {
				log.Warnf("resource search chunk of %d OCID(s) failed after %s: %v", len(chunk), elapsed, err)
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}

			chunkNames := make(map[string]string, len(chunk))
			for _, item := range items {
				if item.Identifier == nil || item.DisplayName == nil || *item.DisplayName == "" {
					continue
				}
				chunkNames[*item.Identifier] = *item.DisplayName
			}

			resolvedAt := time.Now()
			nameCacheMu.Lock()
			for _, ocid := range chunk {
				nameCache[ocid] = nameCacheEntry{name: chunkNames[ocid], resolvedAt: resolvedAt}
			}
			nameCacheMu.Unlock()

			resultsMu.Lock()
			for ocid, name := range chunkNames {
				results[ocid] = name
			}
			resultsMu.Unlock()
			log.Debugf("resource search chunk of %d OCID(s) succeeded in %s, %d resolved with a display name", len(chunk), elapsed, len(chunkNames))
		}(chunk)
	}

	wg.Wait()
	log.Debugf("resource search: resolved %d of %d distinct OCID(s) requested (cache + live)", len(results), len(unique))
	return results, firstErr
}

// searchChunk resolves at most common.MaxIdentifiersPerQuery OCIDs in one or more structured
// queries. Sets Limit explicitly (rather than trusting OCI's undocumented default page size to
// happen to cover the chunk) and follows OpcNextPage until exhausted, so this stays correct even
// if MaxIdentifiersPerQuery is later raised, or a chunk's identifiers happen to match more
// resources than expected -- no assumption baked in that a chunk always fits on one page.
func (r *ResourceSearchResolver) searchChunk(ctx context.Context, ocids []string) ([]resourcesearch.ResourceSummary, error) {
	query := "query all resources where identifier in (" + quotedIdentifierList(ocids) + ")"

	var (
		items []resourcesearch.ResourceSummary
		page  *string
	)

	for {
		req := resourcesearch.SearchResourcesRequest{
			SearchDetails: resourcesearch.StructuredSearchDetails{
				Query: ociCommon.String(query),
			},
			Limit: ociCommon.Int(common.MaxIdentifiersPerQuery),
			Page:  page,
		}

		resp, err := r.fetchPageWithRetry(ctx, req, len(ocids))
		if err != nil {
			return nil, err
		}

		items = append(items, resp.Items...)

		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = resp.OpcNextPage
	}

	return items, nil
}

// fetchPageWithRetry issues req, retrying with exponential backoff and full jitter on 429/5xx
// responses, up to common.MaxSearchRetries extra attempts -- the same retry policy searchChunk
// applied per chunk before pagination support existed, now applied per page.
func (r *ResourceSearchResolver) fetchPageWithRetry(ctx context.Context, req resourcesearch.SearchResourcesRequest, chunkSize int) (resourcesearch.SearchResourcesResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= common.MaxSearchRetries; attempt++ {
		resp, err := r.client.SearchResources(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if !isRetriableSearchError(err) {
			return resourcesearch.SearchResourcesResponse{}, err
		}
		if attempt == common.MaxSearchRetries {
			break
		}

		delay := backoffWithJitter(attempt)
		log.Debugf("resource search chunk of %d OCID(s) hit a retriable error (attempt %d/%d), retrying in %s: %v", chunkSize, attempt+1, common.MaxSearchRetries+1, delay, err)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return resourcesearch.SearchResourcesResponse{}, ctx.Err()
		}
	}

	return resourcesearch.SearchResourcesResponse{}, fmt.Errorf("resource search failed after %d attempt(s): %w", common.MaxSearchRetries+1, lastErr)
}

// isRetriableSearchError reports whether err is a transient OCI service error worth retrying --
// 429 (rate limited) or any 5xx. Everything else (bad request, auth failure, not found) gets
// zero retries, since trying again has no chance of succeeding and only delays shipping logs
// without a name.
func isRetriableSearchError(err error) bool {
	serviceErr, ok := ociCommon.IsServiceError(err)
	if !ok {
		return false
	}
	status := serviceErr.GetHTTPStatusCode()
	return status == http.StatusTooManyRequests || status >= 500
}

// backoffWithJitter implements exponential backoff with full jitter: a random duration between
// 0 and min(common.MaxRetryDelay, common.BaseRetryDelay * 2^attempt).
func backoffWithJitter(attempt int) time.Duration {
	capped := float64(common.BaseRetryDelay) * math.Pow(2, float64(attempt))
	if capped > float64(common.MaxRetryDelay) {
		capped = float64(common.MaxRetryDelay)
	}
	return time.Duration(rand.Float64() * capped)
}

// quotedIdentifierList renders ocids as a single-quoted, comma-separated list for a
// structured-query "in (...)" clause, e.g. 'ocid1...','ocid2...'.
func quotedIdentifierList(ocids []string) string {
	quoted := make([]string, len(ocids))
	for i, ocid := range ocids {
		quoted[i] = "'" + ocid + "'"
	}
	return strings.Join(quoted, ",")
}

// dedupeOCIDs drops empty strings and duplicate OCIDs, preserving first-seen order.
func dedupeOCIDs(ocids []string) []string {
	seen := make(map[string]bool, len(ocids))
	result := make([]string, 0, len(ocids))
	for _, ocid := range ocids {
		if ocid == "" || seen[ocid] {
			continue
		}
		seen[ocid] = true
		result = append(result, ocid)
	}
	return result
}
