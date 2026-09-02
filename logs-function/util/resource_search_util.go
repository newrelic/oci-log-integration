package util

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
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

// NewResourceSearchClient returns a TTL-cached (shares CLIENT_TTL/getClientTTL with
// NewNRClient) OCI Resource Search client, authenticated via Resource Principal -- the same
// mechanism this package already uses for its Secrets client (secrets_util.go), so this feature
// needs no new customer credential flow. Caches on cachedAt being non-zero rather than the
// client being non-nil, deliberately: createResourceSearchClient's two error paths both return a
// real nil, so a nil-pointer check here would never actually cache a failure and would retry on
// every single call.
func NewResourceSearchClient() (ResourceSearchAPI, error) {
	resourceSearchClientMu.Lock()
	defer resourceSearchClientMu.Unlock()

	if !resourceSearchClientCache.IsZero() && time.Since(resourceSearchClientCache) < getClientTTL() {
		return cachedResourceSearchAPI, cachedResourceSearchErr
	}

	cachedResourceSearchAPI, cachedResourceSearchErr = createResourceSearchClient()
	resourceSearchClientCache = time.Now()

	return cachedResourceSearchAPI, cachedResourceSearchErr
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
// dedupes via its pending set, but this stays correct standalone), chunks to
// common.MaxIdentifiersPerQuery (OCI's verified hard limit on an "identifier in (...)" clause),
// and runs chunks concurrently through a semaphore sized common.ResourceSearchWorkerPool. One
// chunk's failure is logged and doesn't block the others -- partial results beat none, the same
// continue-on-error shape ConsumeLogBatches already uses elsewhere in this module. The first
// error encountered (if any) is still returned so the caller knows resolution was incomplete.
func (r *ResourceSearchResolver) ResolveMany(ctx context.Context, ocids []string) (map[string]string, error) {
	unique := dedupeOCIDs(ocids)

	var chunks [][]string
	for i := 0; i < len(unique); i += common.MaxIdentifiersPerQuery {
		end := i + common.MaxIdentifiersPerQuery
		if end > len(unique) {
			end = len(unique)
		}
		chunks = append(chunks, unique[i:end])
	}

	log.Debugf("resource search: resolving %d distinct OCID(s) across %d chunk(s), worker pool=%d", len(unique), len(chunks), common.ResourceSearchWorkerPool)

	results := make(map[string]string)
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

			matched := 0
			resultsMu.Lock()
			for _, item := range items {
				if item.Identifier == nil || item.DisplayName == nil || *item.DisplayName == "" {
					continue
				}
				results[*item.Identifier] = *item.DisplayName
				matched++
			}
			resultsMu.Unlock()
			log.Debugf("resource search chunk of %d OCID(s) succeeded in %s, %d resolved with a display name", len(chunk), elapsed, matched)
		}(chunk)
	}

	wg.Wait()
	log.Debugf("resource search: resolved %d of %d distinct OCID(s) requested", len(results), len(unique))
	return results, firstErr
}

// searchChunk resolves at most common.MaxIdentifiersPerQuery OCIDs in a single structured
// query, retrying with exponential backoff and full jitter on 429/5xx responses. No pagination
// here (unlike a broad "query all resources" sweep) -- an "identifier in (...)" clause over at
// most 20 distinct OCIDs can never match more than 20 resources, well under any page-size
// default, so there's never a next page to follow.
func (r *ResourceSearchResolver) searchChunk(ctx context.Context, ocids []string) ([]resourcesearch.ResourceSummary, error) {
	query := "query all resources where identifier in (" + quotedIdentifierList(ocids) + ")"
	req := resourcesearch.SearchResourcesRequest{
		SearchDetails: resourcesearch.StructuredSearchDetails{
			Query: ociCommon.String(query),
		},
	}

	var lastErr error
	for attempt := 0; attempt <= common.MaxSearchRetries; attempt++ {
		resp, err := r.client.SearchResources(ctx, req)
		if err == nil {
			return resp.Items, nil
		}
		lastErr = err

		if !isRetriableSearchError(err) {
			return nil, err
		}
		if attempt == common.MaxSearchRetries {
			break
		}

		delay := backoffWithJitter(attempt)
		log.Debugf("resource search chunk of %d OCID(s) hit a retriable error (attempt %d/%d), retrying in %s: %v", len(ocids), attempt+1, common.MaxSearchRetries+1, delay, err)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("resource search failed after %d attempt(s): %w", common.MaxSearchRetries+1, lastErr)
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
