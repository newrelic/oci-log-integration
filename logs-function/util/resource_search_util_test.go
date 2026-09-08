package util

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	ociCommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeServiceError satisfies OCI SDK's common.ServiceError interface (structurally -- Go
// interfaces are implicit), so isRetriableSearchError/ociCommon.IsServiceError can be tested
// without a real OCI call.
type fakeServiceError struct {
	status int
}

func (f fakeServiceError) Error() string           { return fmt.Sprintf("fake service error: %d", f.status) }
func (f fakeServiceError) GetHTTPStatusCode() int  { return f.status }
func (f fakeServiceError) GetMessage() string      { return "fake" }
func (f fakeServiceError) GetCode() string         { return "Fake" }
func (f fakeServiceError) GetOpcRequestID() string { return "fake-request-id" }

// mockSearchClient is a flexible, thread-safe test double for ResourceSearchAPI: every call is
// recorded (index and request), and a caller-supplied handler decides the response -- letting
// tests choose whether to key off call order (retry tests) or request content (chunking/
// parallel-aggregation tests).
type mockSearchClient struct {
	mu      sync.Mutex
	calls   int
	reqs    []resourcesearch.SearchResourcesRequest
	handler func(callIndex int, req resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error)
}

func (m *mockSearchClient) SearchResources(ctx context.Context, req resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
	m.mu.Lock()
	idx := m.calls
	m.calls++
	m.reqs = append(m.reqs, req)
	m.mu.Unlock()
	return m.handler(idx, req)
}

func (m *mockSearchClient) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *mockSearchClient) queryOf(idx int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	details := m.reqs[idx].SearchDetails.(resourcesearch.StructuredSearchDetails)
	return *details.Query
}

func summaryWithName(identifier, displayName string) resourcesearch.ResourceSummary {
	return resourcesearch.ResourceSummary{
		Identifier:  ociCommon.String(identifier),
		DisplayName: ociCommon.String(displayName),
	}
}

// --- ResolveMany: chunking, parallel aggregation ---

// TestResolveMany_ChunksAtOCILimit is a regression test for the exact bug this constant fixed
// once before (see common.MaxIdentifiersPerQuery's own doc comment): asserts the literal 20,
// not the constant, so a future edit that reintroduces a too-large value fails here instead of
// only failing against live OCI.
func TestResolveMany_ChunksAtOCILimit(t *testing.T) {
	ocids := make([]string, 45) // expect chunks of 20, 20, 5
	for i := range ocids {
		ocids[i] = fmt.Sprintf("ocid1.instance.oc1..%d", i)
	}

	mock := &mockSearchClient{
		handler: func(_ int, _ resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
			return resourcesearch.SearchResourcesResponse{}, nil
		},
	}
	resolver := NewResourceSearchResolver(mock)

	_, err := resolver.ResolveMany(context.Background(), ocids)
	assert.NoError(t, err)
	assert.Equal(t, 3, mock.callCount(), "45 OCIDs should chunk into 3 queries (20+20+5)")

	sizes := make([]int, mock.callCount())
	for i := 0; i < mock.callCount(); i++ {
		sizes[i] = strings.Count(mock.queryOf(i), ",") + 1
	}
	// Order across chunks isn't guaranteed (they run concurrently), but the three chunk sizes
	// themselves are -- 20, 20, and 5, in some order.
	assert.ElementsMatch(t, []int{20, 20, 5}, sizes)
}

func TestResolveMany_AggregatesDisplayNamesAcrossChunks(t *testing.T) {
	database := map[string]string{
		"ocid1.a.oc1..a": "instance-a",
		"ocid1.b.oc1..b": "instance-b",
		"ocid1.c.oc1..c": "", // present in Resource Search but no display name -- must be skipped
	}
	mock := &mockSearchClient{
		handler: func(_ int, req resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
			details := req.SearchDetails.(resourcesearch.StructuredSearchDetails)
			query := *details.Query
			var items []resourcesearch.ResourceSummary
			for ocid, name := range database {
				if !strings.Contains(query, ocid) {
					continue
				}
				if name == "" {
					items = append(items, resourcesearch.ResourceSummary{Identifier: ociCommon.String(ocid)}) // no DisplayName
					continue
				}
				items = append(items, summaryWithName(ocid, name))
			}
			return resourcesearch.SearchResourcesResponse{ResourceSummaryCollection: resourcesearch.ResourceSummaryCollection{Items: items}}, nil
		},
	}
	resolver := NewResourceSearchResolver(mock)

	resolved, err := resolver.ResolveMany(context.Background(), []string{"ocid1.a.oc1..a", "ocid1.b.oc1..b", "ocid1.c.oc1..c", "ocid1.missing.oc1..z"})
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"ocid1.a.oc1..a": "instance-a",
		"ocid1.b.oc1..b": "instance-b",
	}, resolved, "no-display-name and not-found OCIDs should simply be absent from the result")
}

func TestResolveMany_DedupesInput(t *testing.T) {
	mock := &mockSearchClient{
		handler: func(_ int, req resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
			return resourcesearch.SearchResourcesResponse{}, nil
		},
	}
	resolver := NewResourceSearchResolver(mock)

	_, err := resolver.ResolveMany(context.Background(), []string{"ocid1.a", "ocid1.a", "", "ocid1.a"})
	assert.NoError(t, err)
	assert.Equal(t, 1, mock.callCount(), "a single distinct OCID repeated (and an empty string) should collapse into one chunk")
}

// --- searchChunk: retry, fast-fail, give-up ---

func TestSearchChunk_RetriesOnRetriableErrorThenSucceeds(t *testing.T) {
	mock := &mockSearchClient{
		handler: func(idx int, _ resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
			if idx < 2 {
				return resourcesearch.SearchResourcesResponse{}, fakeServiceError{status: 429}
			}
			return resourcesearch.SearchResourcesResponse{
				ResourceSummaryCollection: resourcesearch.ResourceSummaryCollection{Items: []resourcesearch.ResourceSummary{summaryWithName("ocid1.a", "a")}},
			}, nil
		},
	}
	resolver := NewResourceSearchResolver(mock)

	items, err := resolver.searchChunk(context.Background(), []string{"ocid1.a"})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, 3, mock.callCount(), "two failures then a success should be exactly 3 calls")
}

func TestSearchChunk_FastFailsOnNonRetriableError(t *testing.T) {
	nonRetriable := fakeServiceError{status: 400}
	mock := &mockSearchClient{
		handler: func(_ int, _ resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
			return resourcesearch.SearchResourcesResponse{}, nonRetriable
		},
	}
	resolver := NewResourceSearchResolver(mock)

	_, err := resolver.searchChunk(context.Background(), []string{"ocid1.a"})
	assert.ErrorIs(t, err, nonRetriable)
	assert.Equal(t, 1, mock.callCount(), "a 400 should never be retried")
}

func TestSearchChunk_GivesUpAfterMaxRetries(t *testing.T) {
	mock := &mockSearchClient{
		handler: func(_ int, _ resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
			return resourcesearch.SearchResourcesResponse{}, fakeServiceError{status: 503}
		},
	}
	resolver := NewResourceSearchResolver(mock)

	_, err := resolver.searchChunk(context.Background(), []string{"ocid1.a"})
	assert.Error(t, err)
	assert.Equal(t, common.MaxSearchRetries+1, mock.callCount(), "should attempt exactly MaxSearchRetries+1 times before giving up")
	assert.Contains(t, err.Error(), fmt.Sprintf("failed after %d attempt(s)", common.MaxSearchRetries+1))
}

func TestSearchChunk_ContextCancellationDuringBackoffReturnsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before searchChunk even starts

	mock := &mockSearchClient{
		handler: func(_ int, _ resourcesearch.SearchResourcesRequest) (resourcesearch.SearchResourcesResponse, error) {
			return resourcesearch.SearchResourcesResponse{}, fakeServiceError{status: 500}
		},
	}
	resolver := NewResourceSearchResolver(mock)

	_, err := resolver.searchChunk(ctx, []string{"ocid1.a"})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, mock.callCount(), "the first attempt still runs, but the backoff wait before a retry should exit immediately on a cancelled context")
}

// --- pure unit tests: isRetriableSearchError, backoffWithJitter, dedupeOCIDs ---

func TestIsRetriableSearchError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"429 is retriable", fakeServiceError{status: 429}, true},
		{"500 is retriable", fakeServiceError{status: 500}, true},
		{"503 is retriable", fakeServiceError{status: 503}, true},
		{"400 is not retriable", fakeServiceError{status: 400}, false},
		{"404 is not retriable", fakeServiceError{status: 404}, false},
		{"a non-service error is not retriable", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRetriableSearchError(tt.err))
		})
	}
}

func TestBackoffWithJitter_BoundsAreRespected(t *testing.T) {
	for attempt := 0; attempt < 6; attempt++ {
		expectedCap := float64(common.BaseRetryDelay) * math.Pow(2, float64(attempt))
		if expectedCap > float64(common.MaxRetryDelay) {
			expectedCap = float64(common.MaxRetryDelay)
		}
		for i := 0; i < 20; i++ { // sample repeatedly since it's randomized
			d := backoffWithJitter(attempt)
			assert.GreaterOrEqual(t, d, time.Duration(0))
			assert.LessOrEqual(t, float64(d), expectedCap, "attempt %d delay should never exceed its cap", attempt)
		}
	}
}

func TestDedupeOCIDs(t *testing.T) {
	got := dedupeOCIDs([]string{"ocid1.a", "", "ocid1.b", "ocid1.a", "", "ocid1.c"})
	assert.Equal(t, []string{"ocid1.a", "ocid1.b", "ocid1.c"}, got)
}

// --- NewResourceSearchClient: TTL cache ---

func resetResourceSearchClientCache() {
	resourceSearchClientMu.Lock()
	defer resourceSearchClientMu.Unlock()
	cachedResourceSearchAPI = nil
	cachedResourceSearchErr = nil
	resourceSearchClientCache = time.Time{}
}

// TestNewResourceSearchClient_CacheExpiration mirrors TestNewNRClient_CacheExpiration's intent:
// no real OCI credentials are available in a test environment, so createResourceSearchClient()
// deterministically fails here -- the point is verifying the cache mechanism itself (a failure
// gets cached and reused, per this file's own doc comment on why the cache checks
// cachedAt.IsZero() rather than a nil client), not whether auth succeeds. Since this always
// exercises the failure path, it sets ResourceSearchErrorTTL (the TTL actually applied to a
// cached error), not ClientTTL.
func TestNewResourceSearchClient_CacheExpiration(t *testing.T) {
	resetResourceSearchClientCache()
	os.Setenv(common.ResourceSearchErrorTTL, "1")
	defer os.Unsetenv(common.ResourceSearchErrorTTL)

	_, firstErr := NewResourceSearchClient()
	assert.Error(t, firstErr, "no OCI credentials in a test environment, so this should fail deterministically")
	firstCachedAt := resourceSearchClientCache
	assert.False(t, firstCachedAt.IsZero())

	_, secondErr := NewResourceSearchClient()
	assert.Equal(t, firstErr.Error(), secondErr.Error(), "the cached failure should be returned, not a fresh attempt")
	assert.Equal(t, firstCachedAt, resourceSearchClientCache, "a call within the TTL should not rebuild/re-cache")

	// Simulate TTL expiration by rewinding the cached timestamp directly, rather than sleeping.
	resourceSearchClientMu.Lock()
	resourceSearchClientCache = time.Now().Add(-2 * time.Second)
	resourceSearchClientMu.Unlock()

	_, _ = NewResourceSearchClient()
	assert.True(t, resourceSearchClientCache.After(firstCachedAt), "a call after TTL expiration should rebuild and re-cache")
}

// TestGetResourceSearchErrorTTL mirrors TestGetClientTTL's cases for the error-specific TTL.
func TestGetResourceSearchErrorTTL(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectedTTL time.Duration
	}{
		{name: "Default TTL when no env var", envValue: "", expectedTTL: 30 * time.Second},
		{name: "Custom TTL from env var", envValue: "10", expectedTTL: 10 * time.Second},
		{name: "Invalid TTL falls back to default", envValue: "invalid", expectedTTL: 30 * time.Second},
		{name: "Zero TTL falls back to default", envValue: "0", expectedTTL: 30 * time.Second},
		{name: "Negative TTL falls back to default", envValue: "-5", expectedTTL: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				require.NoError(t, os.Setenv(common.ResourceSearchErrorTTL, tt.envValue))
				defer func() { require.NoError(t, os.Unsetenv(common.ResourceSearchErrorTTL)) }()
			} else {
				require.NoError(t, os.Unsetenv(common.ResourceSearchErrorTTL))
			}

			assert.Equal(t, tt.expectedTTL, getResourceSearchErrorTTL())
		})
	}
}

// TestResourceSearchCacheTTL verifies a cached failure and a cached success are given different
// TTLs -- the fix for the review comment that a transiently-cached auth failure shouldn't be
// stuck for the same window as a healthy cached client.
func TestResourceSearchCacheTTL(t *testing.T) {
	os.Setenv(common.ClientTTL, "600")
	defer os.Unsetenv(common.ClientTTL)
	os.Setenv(common.ResourceSearchErrorTTL, "30")
	defer os.Unsetenv(common.ResourceSearchErrorTTL)

	assert.Equal(t, 600*time.Second, resourceSearchCacheTTL(nil), "a cached success should use the normal client TTL")
	assert.Equal(t, 30*time.Second, resourceSearchCacheTTL(errors.New("boom")), "a cached failure should use the shorter error TTL")
}
