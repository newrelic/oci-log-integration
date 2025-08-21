// Package util provides utility functions for the OCI log integration New Relic client operations.

package util

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test helper function to reset NewRelic client cache
func resetNRClient() {
	cachedNRClient = nil
	nrClientError = nil
	nrClientOnce = sync.Once{}
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
