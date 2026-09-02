package main

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockNewRelicClient is a mock implementation of the NewRelicClientAPI interface
type MockNewRelicClient struct {
	mock.Mock
}

func (m *MockNewRelicClient) CreateLogEntry(logEntry interface{}) error {
	args := m.Called(logEntry)
	return args.Error(0)
}

// TestHandleFunctionWithClient tests the main log processing function
func TestHandleFunctionWithClient(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCalls int
		mockError     error
		expectError   bool
		description   string
	}{
		{
			name: "successful OCI logging event processing",
			input: `[
				{"timestamp":"2023-01-01T12:00:00Z","level":"INFO","message":"Application started","service":"web-server"},
				{"timestamp":"2023-01-01T12:01:00Z","level":"ERROR","message":"Database error","service":"web-server"}
			]`,
			expectedCalls: 1,
			mockError:     nil,
			expectError:   false,
			description:   "Should successfully process valid OCI logging events",
		},
		{
			name:          "single OCI logging event",
			input:         `[{"timestamp":"2023-01-01T12:00:00Z","level":"WARN","message":"High memory usage","service":"api-gateway"}]`,
			expectedCalls: 1,
			mockError:     nil,
			expectError:   false,
			description:   "Should successfully process single OCI logging event",
		},
		{
			name:          "empty log array",
			input:         `[]`,
			expectedCalls: 0,
			mockError:     nil,
			expectError:   false,
			description:   "Should handle empty log arrays without errors",
		},
		{
			name:          "invalid JSON input",
			input:         `{invalid json`,
			expectedCalls: 0,
			mockError:     nil,
			expectError:   true,
			description:   "Should panic on invalid JSON input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockNewRelicClient)

			if tt.expectedCalls > 0 {
				mockClient.On("CreateLogEntry", mock.Anything).Return(tt.mockError).Times(tt.expectedCalls)
			}

			input := bytes.NewReader([]byte(tt.input))
			output := &bytes.Buffer{}

			ctx := context.Background()

			if tt.expectError {
				assert.Panics(t, func() {
					handleFunctionWithClient(ctx, input, output, mockClient)
				}, tt.description)
			} else {
				assert.NotPanics(t, func() {
					handleFunctionWithClient(ctx, input, output, mockClient)

					time.Sleep(100 * time.Millisecond)
				}, tt.description)

				mockClient.AssertExpectations(t)
			}
		})
	}
}

// TestHandleFunctionWithClientConcurrency tests concurrent processing
func TestHandleFunctionWithClientConcurrency(t *testing.T) {
	mockClient := new(MockNewRelicClient)

	mockClient.On("CreateLogEntry", mock.Anything).Return(nil).Maybe()

	input := bytes.NewReader([]byte(`[
		{"timestamp":"2023-01-01T12:00:00Z","level":"INFO","message":"Message 1","service":"service-1"},
		{"timestamp":"2023-01-01T12:00:01Z","level":"INFO","message":"Message 2","service":"service-2"},
		{"timestamp":"2023-01-01T12:00:02Z","level":"INFO","message":"Message 3","service":"service-3"}
	]`))
	output := &bytes.Buffer{}

	ctx := context.Background()

	done := make(chan bool, 1)
	go func() {
		handleFunctionWithClient(ctx, input, output, mockClient)
		done <- true
	}()

	select {
	case <-done:
		assert.True(t, true, "Function completed without hanging")
	case <-time.After(5 * time.Second):
		t.Fatal("Function execution timed out - possible goroutine leak or deadlock")
	}

	time.Sleep(200 * time.Millisecond)

	mockClient.AssertExpectations(t)
}

// TestHandleFunctionErrorCases tests various error scenarios
func TestHandleFunctionErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:        "completely invalid input",
			input:       "not json at all",
			description: "Should panic on non-JSON input",
		},
		{
			name:        "empty input",
			input:       "",
			description: "Should panic on empty input",
		},
		{
			name:        "null input",
			input:       "null",
			description: "Should handle null JSON input gracefully",
		},
		{
			name:        "single object instead of array",
			input:       `{"wrong": "structure"}`,
			description: "Should panic when JSON is object instead of expected array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockNewRelicClient)

			input := bytes.NewReader([]byte(tt.input))
			output := &bytes.Buffer{}

			ctx := context.Background()

			if tt.name == "null input" {
				assert.NotPanics(t, func() {
					handleFunctionWithClient(ctx, input, output, mockClient)
					time.Sleep(50 * time.Millisecond)
				}, tt.description)
				mockClient.AssertExpectations(t)
			} else {
				assert.Panics(t, func() {
					handleFunctionWithClient(ctx, input, output, mockClient)
				}, tt.description)
			}
		})
	}
}

// TestResourceNameEnrichmentEnabled tests the helper that gates the whole enrichment feature,
// matching this codebase's existing DEBUG_ENABLED convention (logger.WithDebugLevel,
// createNRClient): unset or anything other than the exact literal "true" means disabled.
func TestResourceNameEnrichmentEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		unset    bool
		want     bool
	}{
		{name: "unset defaults to disabled", unset: true, want: false},
		{name: "true enables it", envValue: "true", want: true},
		{name: "wrong case does not enable it", envValue: "TRUE", want: false},
		{name: "false stays disabled", envValue: "false", want: false},
		{name: "an arbitrary non-true value stays disabled", envValue: "1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				os.Unsetenv(common.ResourceNameEnrichmentEnabled)
			} else {
				os.Setenv(common.ResourceNameEnrichmentEnabled, tt.envValue)
				defer os.Unsetenv(common.ResourceNameEnrichmentEnabled)
			}
			assert.Equal(t, tt.want, resourceNameEnrichmentEnabled())
		})
	}
}

// TestHandleFunctionWithClient_EnrichmentEnabledButClientUnavailable proves the graceful-fallback
// path: with the flag on, util.NewResourceSearchClient() fails deterministically in this test
// environment (no real OCI Resource Principal credentials available -- same situation
// util.TestNewResourceSearchClient_CacheExpiration already relies on), and log forwarding must
// still succeed, with the original record unenriched, rather than blocking or panicking.
func TestHandleFunctionWithClient_EnrichmentEnabledButClientUnavailable(t *testing.T) {
	os.Setenv(common.ResourceNameEnrichmentEnabled, "true")
	defer os.Unsetenv(common.ResourceNameEnrichmentEnabled)

	mockClient := new(MockNewRelicClient)
	mockClient.On("CreateLogEntry", mock.MatchedBy(func(batch interface{}) bool {
		detailedBatch, ok := batch.(common.DetailedLogsBatch)
		if !ok || len(detailedBatch) == 0 || len(detailedBatch[0].Entries) == 0 {
			return false
		}
		logContent, ok := detailedBatch[0].Entries[0]["logContent"].(map[string]interface{})
		if !ok {
			return false
		}
		data, _ := logContent["data"].(map[string]interface{})
		_, hasInjectedName := data["logging.oci.displayName"]
		return !hasInjectedName // the client failed, so enrichment must not have run at all
	})).Return(nil).Once()

	input := bytes.NewReader([]byte(`[{
		"logContent": {
			"type": "com.oraclecloud.vcn.flowlogs.DataEvent",
			"oracle": {"vnicocid": "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq"},
			"source": "-",
			"data": {}
		}
	}]`))
	output := &bytes.Buffer{}
	ctx := context.Background()

	assert.NotPanics(t, func() {
		handleFunctionWithClient(ctx, input, output, mockClient)
		time.Sleep(100 * time.Millisecond)
	}, "a resource search client failure must not block log forwarding")

	mockClient.AssertExpectations(t)
}
