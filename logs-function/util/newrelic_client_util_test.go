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
	envLicenseKey    string // Environment variable for license key
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
			name:             "Debug enabled with env license key",
			envDebug:         "true",
			envRegion:        "us",
			envLicenseKey:    "valid_license_key",
			expectedLogLevel: "debug",
			expectError:      false,
			description:      "Should work when license key is provided via environment variable",
		},
		{
			name:             "Debug disabled with env license key",
			envRegion:        "us",
			envLicenseKey:    "valid_license_key",
			expectedLogLevel: "info",
			expectError:      false,
			description:      "Should work when license key is provided via environment variable",
		},
		{
			name:          "Invalid region with env license key",
			envRegion:     "invalid",
			envLicenseKey: "valid_license_key",
			expectError:   false,
			description:   "Should handle invalid region gracefully when license key is in env",
		},
		{
			name:           "No license key - should try OCI and fail",
			envRegion:      "us",
			envVaultRegion: "us-ashburn-1",
			envOCID:        "valid_ocid",
			expectError:    true,
			description:    "Should fail when license key is not in env and OCI auth is not available",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup environment variables
			if tc.envDebug != "" {
				os.Setenv(common.DebugEnabled, tc.envDebug)
				defer os.Unsetenv(common.DebugEnabled)
			}
			if tc.envRegion != "" {
				os.Setenv(common.NewRelicRegion, tc.envRegion)
				defer os.Unsetenv(common.NewRelicRegion)
			}
			if tc.envLicenseKey != "" {
				os.Setenv(common.EnvLicenseKey, tc.envLicenseKey)
				defer os.Unsetenv(common.EnvLicenseKey)
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
