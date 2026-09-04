package metrics

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

func resetClientCache() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	cachedClient = nil
	cachedClientErr = nil
	clientCachedAt = time.Time{}
}

func TestNewClient_PropagatesLicenseKeyError(t *testing.T) {
	resetClientCache()
	defer os.Unsetenv(common.NewRelicRegion)
	assert.NoError(t, os.Setenv(common.NewRelicRegion, "us"))

	wantErr := errors.New("secret fetch failed")
	client, err := NewClient(func() (string, error) { return "", wantErr })

	assert.Nil(t, client)
	assert.Equal(t, wantErr, err)
}

func TestNewClient_CachesSuccessfulClient(t *testing.T) {
	resetClientCache()
	defer os.Unsetenv(common.NewRelicRegion)
	assert.NoError(t, os.Setenv(common.NewRelicRegion, "us"))

	calls := 0
	getLicenseKey := func() (string, error) {
		calls++
		return "test-license-key", nil
	}

	client1, err := NewClient(getLicenseKey)
	assert.NoError(t, err)
	assert.NotNil(t, client1)

	client2, err := NewClient(getLicenseKey)
	assert.NoError(t, err)
	assert.Same(t, client1, client2)
	assert.Equal(t, 1, calls, "license key should only be fetched once while cache is valid")
}

// A failed client creation must not be cached for the full (default 10-minute) success-case
// TTL: that would silently block metrics for the whole window on one transient failure.
func TestCacheTTL_CachedErrorUsesShorterNegativeTTL(t *testing.T) {
	defer os.Unsetenv(common.ClientTTL)
	assert.NoError(t, os.Unsetenv(common.ClientTTL)) // default (600s) success-case TTL

	ttl := cacheTTL(errors.New("boom"))

	assert.Equal(t, time.Duration(common.NegativeCacheTTLSeconds)*time.Second, ttl)
}

// An explicitly configured CLIENT_TTL shorter than the negative-cache TTL must still be
// honored for a cached error, rather than being lengthened.
func TestCacheTTL_CachedErrorHonorsShorterConfiguredTTL(t *testing.T) {
	defer os.Unsetenv(common.ClientTTL)
	assert.NoError(t, os.Setenv(common.ClientTTL, "1"))

	assert.Equal(t, 1*time.Second, cacheTTL(errors.New("boom")))
}

// A cached success is unaffected: it keeps the full configured TTL.
func TestCacheTTL_CachedSuccessUsesConfiguredTTL(t *testing.T) {
	defer os.Unsetenv(common.ClientTTL)
	assert.NoError(t, os.Unsetenv(common.ClientTTL))

	assert.Equal(t, clientTTL(), cacheTTL(nil))
}
