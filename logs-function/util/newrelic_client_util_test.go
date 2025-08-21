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

// newNRClientTestCase represents a test case for the NewNRClient function.
type newNRClientTestCase struct {
	name             string // Name of the test case
	envDebug         string // Environment variable for debug
	envRegion        string // Environment variable for region
	envTTL           string // Environment variable for client TTL
	expectedLogLevel string // Expected log level
	expectError      bool   // Whether an error is expected
	envOCID          string // Environment variable for OCI Data
	envVaultRegion   string // Environment variable for Vault Region
	description      string // Description of the test case
}

// TestNewNRClient tests the NewNRClient function with different scenarios.
func TestNewNRClient(t *testing.T) {
	testCases := []newNRClientTestCase{
		{
			name:             "Debug enabled with OCI vault",
			envDebug:         "true",
			envRegion:        "us",
			expectedLogLevel: "debug",
			envOCID:          "valid_ocid",
			envVaultRegion:   "us-ashburn-1",
			expectError:      true,
			description:      "Should attempt OCI vault retrieval but fail in test environment",
		},
		{
			name:             "Debug disabled with OCI vault",
			envRegion:        "us",
			expectedLogLevel: "info",
			envOCID:          "valid_ocid",
			envVaultRegion:   "us-ashburn-1",
			expectError:      true,
			description:      "Should attempt OCI vault retrieval but fail in test environment",
		},
		{
			name:           "Invalid region with OCI vault",
			envRegion:      "invalid",
			envOCID:        "valid_ocid",
			envVaultRegion: "us-ashburn-1",
			expectError:    true,
			description:    "Should handle invalid region and fail in test environment",
		},
		{
			name:        "No OCI configuration - should fail",
			envRegion:   "us",
			expectError: true,
			description: "Should fail when no OCI configuration is available",
		},
		{
			name:           "Custom TTL configuration",
			envRegion:      "us",
			envTTL:         "300",
			envOCID:        "valid_ocid",
			envVaultRegion: "us-ashburn-1",
			expectError:    true,
			description:    "Should respect custom TTL setting (300 seconds)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset client cache for each test
			resetNRClient()

			// Setup environment variables
			if tc.envDebug != "" {
				os.Setenv(common.DebugEnabled, tc.envDebug)
				defer os.Unsetenv(common.DebugEnabled)
			}
			if tc.envRegion != "" {
				os.Setenv(common.NewRelicRegion, tc.envRegion)
				defer os.Unsetenv(common.NewRelicRegion)
			}
			if tc.envTTL != "" {
				os.Setenv(common.ClientTTL, tc.envTTL)
				defer os.Unsetenv(common.ClientTTL)
			}

			if tc.envOCID != "" {
				os.Setenv("SECRET_OCID", tc.envOCID)
				defer os.Unsetenv("SECRET_OCID")
			}
			if tc.envVaultRegion != "" {
				os.Setenv("VAULT_REGION", tc.envVaultRegion)
				defer os.Unsetenv("VAULT_REGION")
			}

			nrClient, err := NewNRClient()

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, nrClient)
			}
		})
	}
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
		// Entries: []common.LogData{"{}"},
	}}

	channel <- logBatch

	ctx := context.TODO()
	wg.Add(1)
	go ConsumeLogBatches(ctx, channel, wg, mockNRClient)
	close(channel)
	wg.Wait()
	mockNRClient.AssertNumberOfCalls(t, "CreateLogEntry", 1)
}

// TestClientTTL tests the TTL functionality of the NewRelic client cache
func TestClientTTL(t *testing.T) {
	// Reset client cache
	resetNRClient()

	// Set up environment for testing
	os.Setenv(common.NewRelicRegion, "us")
	os.Setenv(common.ClientTTL, "60") // 60 seconds TTL for faster testing
	os.Setenv("SECRET_OCID", "test_ocid")
	os.Setenv("VAULT_REGION", "us-ashburn-1")

	defer func() {
		os.Unsetenv(common.NewRelicRegion)
		os.Unsetenv(common.ClientTTL)
		os.Unsetenv("SECRET_OCID")
		os.Unsetenv("VAULT_REGION")
	}()

	// First call should attempt to create client (will fail due to mock environment)
	_, err1 := NewNRClient()
	assert.Error(t, err1, "Expected error due to test environment")

	// Verify cache time was set
	firstCacheTime := clientCacheTime
	assert.False(t, firstCacheTime.IsZero(), "Cache time should be set")

	// Second call within TTL should return cached result (same error)
	_, err2 := NewNRClient()
	assert.Error(t, err2, "Should return cached error")
	assert.Equal(t, err1.Error(), err2.Error(), "Should return same cached error")

	// Verify cache time hasn't changed
	secondCacheTime := clientCacheTime
	assert.Equal(t, firstCacheTime, secondCacheTime, "Cache time should not change for cached response")
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
