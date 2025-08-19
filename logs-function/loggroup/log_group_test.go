package loggroup

import (
	"testing"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

// TestProcessLogs tests the ProcessLogs function
func TestProcessLogs(t *testing.T) {
	tests := []struct {
		name               string
		ociLoggingEvent    common.OCILoggingEvent
		expectedBatches    int
		expectedAttributes map[string]interface{}
		description        string
	}{
		{
			name: "single log entry",
			ociLoggingEvent: common.OCILoggingEvent{
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:00Z",
					"level":     "INFO",
					"message":   "Test message",
					"service":   "test-service",
				},
			},
			expectedBatches: 1,
			expectedAttributes: map[string]interface{}{
				"instrumentation.provider": common.InstrumentationProvider,
				"instrumentation.name":     common.InstrumentationName,
				"instrumentation.version":  common.InstrumentationVersion,
			},
			description: "Should process single log entry and create one batch",
		},
		{
			name: "multiple log entries",
			ociLoggingEvent: common.OCILoggingEvent{
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:00Z",
					"level":     "INFO",
					"message":   "Message 1",
					"service":   "service-1",
				},
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:01Z",
					"level":     "ERROR",
					"message":   "Message 2",
					"service":   "service-2",
				},
				map[string]interface{}{
					"timestamp": "2023-01-01T12:00:02Z",
					"level":     "WARN",
					"message":   "Message 3",
					"service":   "service-3",
				},
			},
			expectedBatches: 1,
			expectedAttributes: map[string]interface{}{
				"instrumentation.provider": common.InstrumentationProvider,
				"instrumentation.name":     common.InstrumentationName,
				"instrumentation.version":  common.InstrumentationVersion,
			},
			description: "Should process multiple log entries into batches",
		},
		{
			name:            "empty log event",
			ociLoggingEvent: common.OCILoggingEvent{},
			expectedBatches: 0,
			expectedAttributes: map[string]interface{}{
				"instrumentation.provider": common.InstrumentationProvider,
				"instrumentation.name":     common.InstrumentationName,
				"instrumentation.version":  common.InstrumentationVersion,
			},
			description: "Should handle empty log events without creating batches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffered channel to capture batches
			channel := make(chan common.DetailedLogsBatch, 10)

			// Call ProcessLogs
			err := ProcessLogs(tt.ociLoggingEvent, channel)

			// Verify no error occurred
			assert.NoError(t, err, tt.description)

			// Close channel and collect all batches
			close(channel)
			var batches []common.DetailedLogsBatch
			for batch := range channel {
				batches = append(batches, batch)
			}

			// Verify expected number of batches
			assert.Len(t, batches, tt.expectedBatches, "Expected %d batches, got %d", tt.expectedBatches, len(batches))

			// Verify attributes if batches were created
			if tt.expectedBatches > 0 && len(batches) > 0 {
				for _, batch := range batches {
					assert.Len(t, batch, 1, "Each batch should contain one DetailedLog")
					detailedLog := batch[0]

					// Verify attributes
					for key, expectedValue := range tt.expectedAttributes {
						actualValue, exists := detailedLog.CommonData.Attributes[key]
						assert.True(t, exists, "Attribute %s should exist", key)
						assert.Equal(t, expectedValue, actualValue, "Attribute %s should have correct value", key)
					}

					// Verify entries
					assert.NotEmpty(t, detailedLog.Entries, "DetailedLog should have entries")
				}
			}
		})
	}
}

// TestSplitLogsIntoBatches tests the splitLogsIntoBatches function
func TestSplitLogsIntoBatches(t *testing.T) {
	tests := []struct {
		name            string
		logs            common.OCILoggingEvent
		maxPayloadSize  int
		expectedBatches int
		description     string
	}{
		{
			name: "single small log fits in one batch",
			logs: common.OCILoggingEvent{
				map[string]interface{}{
					"level":   "INFO",
					"message": "Small message",
				},
			},
			maxPayloadSize:  1000,
			expectedBatches: 1,
			description:     "Single small log should fit in one batch",
		},
		{
			name: "multiple small logs fit in one batch",
			logs: common.OCILoggingEvent{
				map[string]interface{}{
					"level":   "INFO",
					"message": "Message 1",
				},
				map[string]interface{}{
					"level":   "INFO",
					"message": "Message 2",
				},
				map[string]interface{}{
					"level":   "INFO",
					"message": "Message 3",
				},
			},
			maxPayloadSize:  1000,
			expectedBatches: 1,
			description:     "Multiple small logs should fit in one batch",
		},
		{
			name: "logs exceed max payload size create multiple batches",
			logs: common.OCILoggingEvent{
				map[string]interface{}{
					"level":   "INFO",
					"message": "This is a longer message that will help us test payload size limits and batching behavior",
				},
				map[string]interface{}{
					"level":   "ERROR",
					"message": "Another longer message to test batching when payload size is exceeded",
				},
				map[string]interface{}{
					"level":   "WARN",
					"message": "Yet another message to ensure we create multiple batches",
				},
			},
			maxPayloadSize:  100, // Small size to force multiple batches
			expectedBatches: 3,   // Each log will be in its own batch
			description:     "Logs exceeding payload size should create multiple batches",
		},
		{
			name:            "empty logs create no batches",
			logs:            common.OCILoggingEvent{},
			maxPayloadSize:  1000,
			expectedBatches: 0,
			description:     "Empty logs should create no batches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffered channel to capture batches
			channel := make(chan common.DetailedLogsBatch, 10)

			// Create common attributes
			commonAttributes := common.LogAttributes{
				"test.attribute": "test.value",
			}

			// Call splitLogsIntoBatches
			err := splitLogsIntoBatches(tt.logs, tt.maxPayloadSize, commonAttributes, channel)

			// Verify no error occurred
			assert.NoError(t, err, tt.description)

			// Close channel and collect all batches
			close(channel)
			var batches []common.DetailedLogsBatch
			for batch := range channel {
				batches = append(batches, batch)
			}

			// Verify expected number of batches
			assert.Len(t, batches, tt.expectedBatches, "Expected %d batches, got %d", tt.expectedBatches, len(batches))

			// Verify all logs are included across batches
			if tt.expectedBatches > 0 {
				totalLogs := 0
				for _, batch := range batches {
					assert.Len(t, batch, 1, "Each batch should contain one DetailedLog")
					detailedLog := batch[0]
					totalLogs += len(detailedLog.Entries)

					// Verify attributes are set
					assert.Equal(t, "test.value", detailedLog.CommonData.Attributes["test.attribute"])
				}
				assert.Equal(t, len(tt.logs), totalLogs, "All logs should be included across batches")
			}
		})
	}
}

// TestSplitLogsIntoBatchesPayloadSizeAccuracy tests payload size calculation accuracy
func TestSplitLogsIntoBatchesPayloadSizeAccuracy(t *testing.T) {
	// Create logs with known sizes
	logs := common.OCILoggingEvent{
		map[string]interface{}{
			"msg": "a",
		}, // Small log
		map[string]interface{}{
			"message": "This is a medium-sized log entry that should fit within reasonable payload limits",
		}, // Medium log
	}

	channel := make(chan common.DetailedLogsBatch, 10)
	commonAttributes := common.LogAttributes{
		"test": "value",
	}

	// Use a size that should allow the first log but not both
	err := splitLogsIntoBatches(logs, 50, commonAttributes, channel)
	assert.NoError(t, err)

	close(channel)
	var batches []common.DetailedLogsBatch
	for batch := range channel {
		batches = append(batches, batch)
	}

	// Should create 2 batches since the second log exceeds the size limit
	assert.Len(t, batches, 2, "Should create 2 batches due to payload size limits")
}

// TestProcessLogsWithChannel tests the channel communication
func TestProcessLogsWithChannel(t *testing.T) {
	logs := common.OCILoggingEvent{
		map[string]interface{}{
			"timestamp": "2023-01-01T12:00:00Z",
			"level":     "INFO",
			"message":   "Test message 1",
		},
		map[string]interface{}{
			"timestamp": "2023-01-01T12:00:01Z",
			"level":     "ERROR",
			"message":   "Test message 2",
		},
	}

	channel := make(chan common.DetailedLogsBatch, 5)

	// Process logs
	err := ProcessLogs(logs, channel)
	assert.NoError(t, err)

	// Test that we can receive from the channel without blocking
	select {
	case batch := <-channel:
		assert.NotEmpty(t, batch, "Should receive a non-empty batch")
		assert.Len(t, batch, 1, "Batch should contain one DetailedLog")

		detailedLog := batch[0]
		assert.Equal(t, common.InstrumentationProvider, detailedLog.CommonData.Attributes["instrumentation.provider"])
		assert.Equal(t, common.InstrumentationName, detailedLog.CommonData.Attributes["instrumentation.name"])
		assert.Equal(t, common.InstrumentationVersion, detailedLog.CommonData.Attributes["instrumentation.version"])
		assert.Len(t, detailedLog.Entries, 2, "Should contain both log entries")

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for batch from channel")
	}

	close(channel)
}

// TestProcessLogsAttributes tests that correct attributes are set
func TestProcessLogsAttributes(t *testing.T) {
	logs := common.OCILoggingEvent{
		map[string]interface{}{
			"level":   "INFO",
			"message": "Test",
		},
	}

	channel := make(chan common.DetailedLogsBatch, 1)

	err := ProcessLogs(logs, channel)
	assert.NoError(t, err)

	close(channel)
	batch := <-channel

	assert.Len(t, batch, 1)
	detailedLog := batch[0]

	// Verify all expected instrumentation attributes
	expectedAttributes := map[string]interface{}{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	for key, expectedValue := range expectedAttributes {
		actualValue, exists := detailedLog.CommonData.Attributes[key]
		assert.True(t, exists, "Attribute %s should exist", key)
		assert.Equal(t, expectedValue, actualValue, "Attribute %s should have correct value", key)
	}

	// Verify only expected attributes are present (no extra attributes)
	assert.Len(t, detailedLog.CommonData.Attributes, len(expectedAttributes), "Should only have expected attributes")
}
