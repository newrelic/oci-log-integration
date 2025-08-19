// Package loggroup provides functionality for processing and batching OCI log events
// for efficient transmission to New Relic's logging API.
package loggroup

import (
	"encoding/json"
	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/newrelic/oci-log-integration/logs-function/util"
	// "time"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// ProcessLogs processes OCI logging events and splits them into batches for New Relic ingestion.
// It adds instrumentation metadata to each batch and sends the batches through the provided channel.
// The function respects payload size limits to ensure compatibility with New Relic's API constraints.
func ProcessLogs(OCILoggingEvent common.OCILoggingEvent, channel chan common.DetailedLogsBatch) error {
	attributes := common.LogAttributes{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	splitLogsIntoBatches(OCILoggingEvent, common.MaxPayloadSize, attributes, channel)

	return nil
}

func splitLogsIntoBatches(logs common.OCILoggingEvent, maxPayloadSize int, commonAttributes common.LogAttributes, channel chan common.DetailedLogsBatch) error {
	var currentBatch common.LogData
	currentBatchSize := 0

	for _, logData := range logs {
		logBytes, err := json.Marshal(logData)
		if err != nil {
			log.Debugf("Warning: Could not marshal detailed log for size estimation: %v", err)
			continue
		}
		logSize := len(logBytes)

		if currentBatchSize+logSize > maxPayloadSize && len(currentBatch) > 0 {
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
			util.ProduceMessageToChannel(channel, currentBatch, commonAttributes)
		} else {
			currentBatch = append(currentBatch, logData)
			currentBatchSize += logSize
		}
	}

	if len(currentBatch) > 0 {
		util.ProduceMessageToChannel(channel, currentBatch, commonAttributes)
	}

	return nil
}
