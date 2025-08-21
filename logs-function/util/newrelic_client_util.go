// Package util provides utility functions for New Relic client operations,
// secret management, and message processing for the OCI log integration.
package util

import (
	"context"
	"os"
	"sync"

	"github.com/newrelic/newrelic-client-go/v2/pkg/config"
	logging "github.com/newrelic/newrelic-client-go/v2/pkg/logs"
	"github.com/newrelic/newrelic-client-go/v2/pkg/region"
	
	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// Global variables for caching the NewRelic client
var (
	cachedNRClient   NewRelicClientAPI
	nrClientOnce     sync.Once
	nrClientError    error
)

// NewRelicClientAPI is an interface that defines the methods for interacting with the New Relic Logs API.
type NewRelicClientAPI interface {
	CreateLogEntry(logEntry interface{}) error
}

// ConsumeLogBatches consumes log batches from a channel and creates log entries using the provided NewRelicClientAPI.
// The function returns when the channel is closed or the context is cancelled.
func ConsumeLogBatches(ctx context.Context, channel <-chan common.DetailedLogsBatch, wg *sync.WaitGroup, nrClientAPI NewRelicClientAPI) {
	// Defer the Done() method of the WaitGroup to indicate that the goroutine has finished processing
	defer wg.Done()

	for {
		select {
		case batch, ok := <-channel:
			if !ok {
				return
			}
			if err := nrClientAPI.CreateLogEntry(batch); err != nil {
				log.Errorf("error posting Log entry: %v", err)
				// Continue processing other batches instead of terminating
				continue
			}
		case <-ctx.Done():
			// Context has been cancelled, exit the goroutine
			return
		}
	}
}

// NewNRClient Initializes a new NRClient with debug level and region
// It returns a NewRelicClientAPI interface and an error if there is a problem setting the region.
// Uses lazy initialization with caching for performance.
func NewNRClient() (NewRelicClientAPI, error) {
	nrClientOnce.Do(func() {
		log.Debug("Initializing New Relic client (lazy initialization)")
		cachedNRClient, nrClientError = createNRClient()
		if nrClientError == nil {
			log.Debug("New Relic client initialized successfully")
		}
	})
	return cachedNRClient, nrClientError
}

// createNRClient creates a new NewRelic client instance
func createNRClient() (NewRelicClientAPI, error) {
	nrRegion, _ := region.Get(region.Name(os.Getenv(common.NewRelicRegion)))
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
		return &nrClient, err
	}

	licenseKey, err := GetLicenseKey()
	cfg.LicenseKey = licenseKey
	nrClient = logging.New(cfg)
	return &nrClient, err
}
