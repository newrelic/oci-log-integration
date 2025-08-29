// Package util provides utility functions for the OCI log integration New Relic client operations.

package util

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test helper function to reset NewRelic client cache
func resetNRClient() {
	cachedNRClient = nil
	nrClientError = nil
	clientCacheTime = time.Time{}
}

// MockNRClient is a mock type for the Logs interface.
type MockNRClient struct {
	mock.Mock
}

// CreateLogEntry is a mock method that satisfies the Logs interface.
func (m *MockNRClient) CreateLogEntry(batch interface{}) error {
	args := m.Called(batch)
	return args.Error(0)
}

// TestConsumeLogBatches tests the ConsumeLogBatches function.
func TestConsumeLogBatches(t *testing.T) {
	mockNRClient := new(MockNRClient)
	mockNRClient.On("CreateLogEntry", mock.Anything).Return(nil)

	channel := make(chan common.DetailedLogsBatch, 1)
	wg := new(sync.WaitGroup)

	logBatch := []common.DetailedLog{{
		CommonData: common.Common{
			Attributes: common.LogAttributes{
				"compartmentId": "ocid1.compartment.oc1..aaaaaaaa",
				"tenantId":      "ocid1.tenancy.oc1..bbbbbbbbb",
				"region":        "us-ashburn-1",
			},
		},
	}}

	channel <- logBatch

	ctx := context.TODO()
	wg.Add(1)
	go ConsumeLogBatches(ctx, channel, wg, mockNRClient)
	close(channel)
	wg.Wait()
	mockNRClient.AssertNumberOfCalls(t, "CreateLogEntry", 1)
}

// TestGetClientTTL tests the getClientTTL function
func TestGetClientTTL(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectedTTL time.Duration
	}{
		{
			name:        "Default TTL when no env var",
			envValue:    "",
			expectedTTL: 600 * time.Second,
		},
		{
			name:        "Custom TTL from env var",
			envValue:    "300",
			expectedTTL: 300 * time.Second,
		},
		{
			name:        "Invalid TTL falls back to default",
			envValue:    "invalid",
			expectedTTL: 600 * time.Second,
		},
		{
			name:        "Zero TTL falls back to default",
			envValue:    "0",
			expectedTTL: 600 * time.Second,
		},
		{
			name:        "Negative TTL falls back to default",
			envValue:    "-5",
			expectedTTL: 600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(common.ClientTTL, tt.envValue)
				defer os.Unsetenv(common.ClientTTL)
			} else {
				os.Unsetenv(common.ClientTTL)
			}

			actualTTL := getClientTTL()
			assert.Equal(t, tt.expectedTTL, actualTTL)
		})
	}
}

// TestNewNRClient_CacheLogic tests the caching logic of NewNRClient
func TestNewNRClient_CacheLogic(t *testing.T) {
	resetNRClient()

	os.Setenv(common.NewRelicRegion, "us")
	os.Setenv(common.ClientTTL, "60")
	defer func() {
		os.Unsetenv(common.NewRelicRegion)
		os.Unsetenv(common.ClientTTL)
	}()

	_, _ = NewNRClient()
	firstCacheTime := clientCacheTime
	assert.False(t, firstCacheTime.IsZero(), "Cache time should be set after first call")

	_, _ = NewNRClient()
	secondCacheTime := clientCacheTime
	assert.Equal(t, firstCacheTime, secondCacheTime, "Cache time should not change for cached response")
}

// TestNewNRClient_CacheExpiration tests that cache expires correctly
func TestNewNRClient_CacheExpiration(t *testing.T) {
	resetNRClient()

	os.Setenv(common.ClientTTL, "1")
	os.Setenv(common.NewRelicRegion, "us")
	defer func() {
		os.Unsetenv(common.ClientTTL)
		os.Unsetenv(common.NewRelicRegion)
	}()

	_, _ = NewNRClient()
	firstCacheTime := clientCacheTime

	time.Sleep(2 * time.Second)

	_, _ = NewNRClient()
	secondCacheTime := clientCacheTime

	assert.True(t, secondCacheTime.After(firstCacheTime), "Cache should have been refreshed after TTL expiration")
}

// TestConsumeLogBatches_ErrorHandling tests error handling in log processing
func TestConsumeLogBatches_ErrorHandling(t *testing.T) {
	mockNRClient := new(MockNRClient)
	
	mockNRClient.On("CreateLogEntry", mock.Anything).Return(assert.AnError)

	channel := make(chan common.DetailedLogsBatch, 2)
	wg := new(sync.WaitGroup)

	// Send two batches
	logBatch1 := []common.DetailedLog{{
		CommonData: common.Common{
			Attributes: common.LogAttributes{
				"compartmentId": "ocid1.compartment.oc1..aaaaaaaa",
			},
		},
	}}
	logBatch2 := []common.DetailedLog{{
		CommonData: common.Common{
			Attributes: common.LogAttributes{
				"compartmentId": "ocid1.compartment.oc1..bbbbbbbbb",
			},
		},
	}}

	channel <- logBatch1
	channel <- logBatch2

	ctx := context.TODO()
	wg.Add(1)
	go ConsumeLogBatches(ctx, channel, wg, mockNRClient)
	close(channel)
	wg.Wait()
	
	mockNRClient.AssertNumberOfCalls(t, "CreateLogEntry", 2)
}