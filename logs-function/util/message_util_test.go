package util

import (
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

// TestProduceMessageToChannel tests the ProduceMessageToChannel function
func TestProduceMessageToChannel(t *testing.T) {
	// Create a channel for DetailedLogsBatch
	channel := make(chan common.DetailedLogsBatch, 1) // Buffered channel

	// Create a sample log data and attributes
	currentBatch := common.LogData{
		map[string]interface{}{
			"message": map[string]interface{}{
				"level": "info",
				"text":  "test log message 1",
			},
		},
		map[string]interface{}{
			"message": map[string]interface{}{
				"level": "error",
				"text":  "test log message 2",
			},
		},
	}

	attributes := common.LogAttributes{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	expectedDetailedLog := common.DetailedLogsBatch{{
		CommonData: common.Common{
			Attributes: attributes,
		},
		Entries: currentBatch,
	}}
	ProduceMessageToChannel(channel, currentBatch, attributes)
	receivedDetailedLog := <-channel

	assert.Equal(t, expectedDetailedLog, receivedDetailedLog)

	// Close the channel
	close(channel)
}
