// Package util provides utility functions for New Relic client operations,
// secret management, and message processing for the OCI log integration.
package util

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/newrelic/newrelic-client-go/v2/pkg/config"
	logging "github.com/newrelic/newrelic-client-go/v2/pkg/logs"
	"github.com/newrelic/newrelic-client-go/v2/pkg/region"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/metrics"
)

// Global variables for caching the NewRelic client with TTL support
var (
	cachedNRClient  NewRelicClientAPI
	nrClientError   error
	clientCacheTime time.Time
)

// NewRelicClientAPI is an interface that defines the methods for interacting with the New Relic Logs API.
type NewRelicClientAPI interface {
	CreateLogEntry(logEntry interface{}) error
}

// ConsumeLogBatches consumes log batches from a channel and creates log entries using the provided NewRelicClientAPI.
// The function returns when the channel is closed or the context is cancelled. rec may be nil.
func ConsumeLogBatches(ctx context.Context, channel <-chan BatchMessage, wg *sync.WaitGroup, nrClientAPI NewRelicClientAPI, rec *metrics.Recorder) {
	// Defer the Done() method of the WaitGroup to indicate that the goroutine has finished processing
	defer wg.Done()

	for {
		select {
		case msg, ok := <-channel:
			if !ok {
				return
			}
			batch := msg.Batch

			recordCount := countBatchEntries(batch)

			start := time.Now()
			err := nrClientAPI.CreateLogEntry(batch)
			duration := time.Since(start).Seconds()

			if err != nil {
				log.Errorf("error posting Log entry: %v", err)
				rec.Summary(metrics.Basic.MetricDeliveryDuration, duration, map[string]interface{}{"status": "error"})
				rec.Count(metrics.Basic.MetricRecordsDropped, float64(recordCount), map[string]interface{}{"reason": "delivery_error"})
				rec.Count(metrics.Advanced.MetricDeliveryErrors, 1, map[string]interface{}{"error_class": fmt.Sprintf("%T", err), "status": "error"})
				// Continue processing other batches instead of terminating
				continue
			}

			rec.Summary(metrics.Basic.MetricDeliveryDuration, duration, map[string]interface{}{"status": "success"})
			rec.Count(metrics.Basic.MetricRecordsDelivered, float64(recordCount), map[string]interface{}{"status": "success"})
			// SizeBytes was already computed once while building the batch (loggroup);
			// reuse it here instead of re-marshaling the whole batch just to measure it.
			rec.Count(metrics.Advanced.MetricBytesDelivered, float64(msg.SizeBytes), nil)
		case <-ctx.Done():
			// Context has been cancelled, exit the goroutine
			return
		}
	}
}

// countBatchEntries counts the total number of log records across all DetailedLogs in a batch.
func countBatchEntries(batch common.DetailedLogsBatch) int {
	total := 0
	for _, detailedLog := range batch {
		total += len(detailedLog.Entries)
	}
	return total
}

// NewNRClient Initializes a new NRClient with debug level and region
// It returns a NewRelicClientAPI interface and an error if there is a problem setting the region.
// Uses TTL-based caching for performance in OCI Function environment. A cached failure is held
// for only NegativeCacheTTLSeconds (much shorter than the success-case TTL), so a transient
// error doesn't get replayed -- and re-increment failure metrics -- for the full TTL. rec may be nil.
func NewNRClient(rec *metrics.Recorder) (NewRelicClientAPI, error) {
	// Check if cache is still valid
	if !clientCacheTime.IsZero() && time.Since(clientCacheTime) < cacheTTL(nrClientError) {
		// Return cached client (even if there was an error before)
		log.Debug("Returning cached New Relic client")
		rec.Count(metrics.Advanced.MetricClientCache, 1, map[string]interface{}{"result": "hit"})
		return cachedNRClient, nrClientError
	}

	// Cache is invalid, expired, or doesn't exist - create new client
	log.Debug("Initializing/refreshing New Relic client")
	rec.Count(metrics.Advanced.MetricClientCache, 1, map[string]interface{}{"result": "miss"})
	cachedNRClient, nrClientError = createNRClient()
	clientCacheTime = time.Now()

	if nrClientError == nil {
		log.Debug("New Relic client initialized successfully")
	}

	return cachedNRClient, nrClientError
}

// getClientTTL returns the TTL for the client cache from environment variable or default (600 seconds = 10 minutes)
func getClientTTL() time.Duration {
	ttlSeconds := common.DefaultClientTTL // Default TTL in seconds

	if envTTL := os.Getenv(common.ClientTTL); envTTL != "" {
		if parsedTTL, err := strconv.Atoi(envTTL); err == nil && parsedTTL > 0 {
			ttlSeconds = parsedTTL
		}
	}

	return time.Duration(ttlSeconds) * time.Second
}

// clientErrorClass is the reason NewNRClient/createNRClient failed to build a client, so
// callers can label observability metrics by actual cause instead of lumping a region
// misconfig and a Vault/license-key fetch failure into one bucket.
type clientErrorClass string

const (
	clientErrorClassRegion clientErrorClass = "region_config"
	clientErrorClassSecret clientErrorClass = "secret_fetch"
)

// classifiedError associates a createNRClient failure with the stage that caused it.
type classifiedError struct {
	class clientErrorClass
	err   error
}

func (e *classifiedError) Error() string { return e.err.Error() }
func (e *classifiedError) Unwrap() error { return e.err }

// NRClientErrorClass returns a stable label for why err (as returned by NewNRClient) failed,
// for use as an "error_class" metric attribute. Returns "unknown" for a nil or unrecognized
// error.
func NRClientErrorClass(err error) string {
	var ce *classifiedError
	if errors.As(err, &ce) {
		return string(ce.class)
	}
	return "unknown"
}

// createNRClient creates a new NewRelic client instance
func createNRClient() (NewRelicClientAPI, error) {
	nrRegion, err := region.Get(region.Name(os.Getenv(common.NewRelicRegion)))
	if err != nil {
		log.Warnf("could not resolve NEW_RELIC_REGION %q, falling back to default region: %v", os.Getenv(common.NewRelicRegion), err)
	}
	var nrClient logging.Logs
	cfg := config.Config{
		Compression: config.Compression.Gzip,
	}

	if os.Getenv(common.DebugEnabled) == "true" {
		cfg.LogLevel = "debug"
	} else {
		cfg.LogLevel = "info"
	}

	if err := cfg.SetRegion(nrRegion); err != nil {
		return &nrClient, &classifiedError{class: clientErrorClassRegion, err: err}
	}

	licenseKey, err := GetLicenseKey()
	if err != nil {
		return &nrClient, &classifiedError{class: clientErrorClassSecret, err: err}
	}
	cfg.LicenseKey = licenseKey
	nrClient = logging.New(cfg)
	return &nrClient, nil
}
