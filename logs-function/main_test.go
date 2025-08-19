package main

import (
	"bytes"
	"context"
	"testing"
	"time"

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
			expectError:   false,
			description:   "Should handle invalid JSON gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := new(MockNewRelicClient)

			// Set up expectations based on test case
			if tt.expectedCalls > 0 {
				mockClient.On("CreateLogEntry", mock.Anything).Return(tt.mockError).Times(tt.expectedCalls)
			}

			// Create input reader
			input := bytes.NewReader([]byte(tt.input))
			output := &bytes.Buffer{}

			// Call the function under test
			ctx := context.Background()

			// The function should not panic
			assert.NotPanics(t, func() {
				handleFunctionWithClient(ctx, input, output, mockClient)

				// Give some time for goroutines to complete
				time.Sleep(100 * time.Millisecond)
			}, tt.description)

			// Verify mock expectations
			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleFunctionWithClientConcurrency tests concurrent processing
func TestHandleFunctionWithClientConcurrency(t *testing.T) {
	// Create mock client
	mockClient := new(MockNewRelicClient)

	// Set up expectation for log processing
	mockClient.On("CreateLogEntry", mock.Anything).Return(nil).Maybe()

	// Create input with multiple log entries
	input := bytes.NewReader([]byte(`[
		{"timestamp":"2023-01-01T12:00:00Z","level":"INFO","message":"Message 1","service":"service-1"},
		{"timestamp":"2023-01-01T12:00:01Z","level":"INFO","message":"Message 2","service":"service-2"},
		{"timestamp":"2023-01-01T12:00:02Z","level":"INFO","message":"Message 3","service":"service-3"}
	]`))
	output := &bytes.Buffer{}

	ctx := context.Background()

	// Test that function completes without hanging
	done := make(chan bool, 1)
	go func() {
		handleFunctionWithClient(ctx, input, output, mockClient)
		done <- true
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// Function completed successfully
		assert.True(t, true, "Function completed without hanging")
	case <-time.After(5 * time.Second):
		t.Fatal("Function execution timed out - possible goroutine leak or deadlock")
	}

	// Give time for any background goroutines to complete
	time.Sleep(200 * time.Millisecond)

	mockClient.AssertExpectations(t)
}

// TestHandleFunctionWithClientContextCancellation tests context cancellation handling
func TestHandleFunctionWithClientContextCancellation(t *testing.T) {
	// Create mock client
	mockClient := new(MockNewRelicClient)

	// Mock can be called but may not be due to cancellation
	mockClient.On("CreateLogEntry", mock.Anything).Return(nil).Maybe()

	// Create input
	input := bytes.NewReader([]byte(`[
		{"timestamp":"2023-01-01T12:00:00Z","level":"INFO","message":"Test message","service":"test-service"}
	]`))
	output := &bytes.Buffer{}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start the function
	done := make(chan bool, 1)
	go func() {
		handleFunctionWithClient(ctx, input, output, mockClient)
		done <- true
	}()

	// Cancel context after a short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for completion
	select {
	case <-done:
		assert.True(t, true, "Function handled context cancellation gracefully")
	case <-time.After(2 * time.Second):
		t.Fatal("Function did not handle context cancellation within timeout")
	}

	// Note: We don't assert expectations here since cancellation timing affects execution
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
			description: "Should handle non-JSON input gracefully",
		},
		{
			name:        "empty input",
			input:       "",
			description: "Should handle empty input gracefully",
		},
		{
			name:        "null input",
			input:       "null",
			description: "Should handle null JSON input gracefully",
		},
		{
			name:        "wrong JSON structure",
			input:       `{"wrong": "structure"}`,
			description: "Should handle incorrect JSON structure gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := new(MockNewRelicClient)

			// No calls expected for invalid input
			// mockClient.On("CreateLogEntry", mock.Anything).Return(nil).Maybe()

			// Create input reader
			input := bytes.NewReader([]byte(tt.input))
			output := &bytes.Buffer{}

			// Call the function under test
			ctx := context.Background()

			// The function should not panic even with invalid input
			assert.NotPanics(t, func() {
				handleFunctionWithClient(ctx, input, output, mockClient)

				// Give some time for any processing
				time.Sleep(50 * time.Millisecond)
			}, tt.description)

			// Verify no unexpected calls were made
			mockClient.AssertExpectations(t)
		})
	}
}
