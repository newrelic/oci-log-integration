// Package loggroup provides functionality for processing and batching OCI log events
// for efficient transmission to New Relic's logging API.
package loggroup

import (
	"context"
	"encoding/json"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/newrelic/oci-log-integration/logs-function/metrics"
	"github.com/newrelic/oci-log-integration/logs-function/util"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// ProcessLogs processes OCI logging events and splits them into batches for New Relic ingestion.
// It adds instrumentation metadata to each batch and sends the batches through the provided channel.
// The function respects payload size limits to ensure compatibility with New Relic's API constraints.
// metricRecorder may be nil.
func ProcessLogs(ctx context.Context, OCILoggingEvent common.OCILoggingEvent, channel chan util.BatchMessage, metricRecorder *metrics.Recorder) {
	attributes := common.LogAttributes{
		"instrumentation.provider": common.InstrumentationProvider,
		"instrumentation.name":     common.InstrumentationName,
		"instrumentation.version":  common.InstrumentationVersion,
	}

	splitLogsIntoBatches(ctx, OCILoggingEvent, common.MaxPayloadSize, attributes, channel, metricRecorder)
}

// splitLogsIntoBatches splits the incoming logs into batches for processing.
// It loosely respects (if a single log entry exceeds the maximum payload size we still try to send it)
// the maximum payload size and sends each batch through the provided channel. It stops early if
// ctx is cancelled while trying to hand a batch to the (full) channel, rather than blocking
// forever on a stuck consumer.
func splitLogsIntoBatches(ctx context.Context, logs common.OCILoggingEvent, maxPayloadSize int, commonAttributes common.LogAttributes, channel chan util.BatchMessage, metricRecorder *metrics.Recorder) {
	var currentBatch common.LogData
	currentBatchSize := 0

	for i, logData := range logs {
		if env := metrics.ExtractEnvelope(logData); env.HasLagTime {
			lag := time.Since(env.LagTime).Seconds()
			// Lag is anchored on OCI's ingestion time (see ExtractEnvelope), so it should be
			// non-negative. A negative value can still occur on the source-time fallback path
			// (a source clock ahead of the forwarder host) or from residual clock skew, and is
			// not physically meaningful as pipeline latency. Clamp the latency observation to 0
			// so it can't drag the pipeline-lag average/min negative, and count the occurrence
			// separately so the skew stays visible instead of being silently hidden.
			if lag < 0 {
				metricRecorder.Count(metrics.TierBasic, metrics.MetricPipelineLagNegative, 1, nil)
				lag = 0
			}
			metricRecorder.Summary(metrics.TierBasic, metrics.MetricPipelineLag, lag, nil)
		}

		logBytes, err := json.Marshal(logData)
		if err != nil {
			metricRecorder.Count(metrics.TierAdvanced, metrics.MetricSerializeErrors, 1, nil)
			log.Warnf("Warning: Could not marshal detailed log for size estimation: %v", err)
			continue
		}
		logSize := len(logBytes)

		// OCI has a 1MB limit per log line; a single entry that alone exceeds maxPayloadSize
		// is still pushed to New Relic (see below), but flagged here. Checked unconditionally
		// per record rather than only when a record happens to start a new batch -- otherwise
		// an oversized record arriving after another batch was already flushed would slip
		// through uncounted.
		if logSize > maxPayloadSize {
			metricRecorder.Count(metrics.TierAdvanced, metrics.MetricRecordsOversized, 1, nil)
		}

		if len(currentBatch) == 0 {
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
		} else if currentBatchSize+logSize > maxPayloadSize && len(currentBatch) > 0 {
			if !produceBatch(ctx, channel, currentBatch, commonAttributes, currentBatchSize, metricRecorder) {
				// Everything from here on (this record onward) never made it into a batch at
				// all, so it needs its own drop accounting on top of what produceBatch already
				// recorded for the batch it failed to send.
				remaining := len(logs) - i
				metricRecorder.Count(metrics.TierBasic, metrics.MetricRecordsDropped, float64(remaining), map[string]interface{}{"reason": "producer_cancelled"})
				log.Warnf("context cancelled while producing log batch; dropping %d remaining log record(s)", remaining)
				return
			}
			currentBatch = common.LogData{logData}
			currentBatchSize = logSize
		} else {
			currentBatch = append(currentBatch, logData)
			currentBatchSize += logSize
		}
	}

	if len(currentBatch) > 0 {
		produceBatch(ctx, channel, currentBatch, commonAttributes, currentBatchSize, metricRecorder)
	}
}

// produceBatch sends a completed batch to the channel and records its advanced-tier batching
// metrics. It returns false if ctx is cancelled before the batch could be handed off -- e.g. all
// consumer workers are stuck on a slow New Relic API call and the channel buffer is full --
// recording the batch's records as dropped instead of blocking forever.
func produceBatch(ctx context.Context, channel chan util.BatchMessage, batch common.LogData, commonAttributes common.LogAttributes, batchSize int, metricRecorder *metrics.Recorder) bool {
	if !util.ProduceMessageToChannel(ctx, channel, batch, commonAttributes, batchSize) {
		metricRecorder.Count(metrics.TierBasic, metrics.MetricRecordsDropped, float64(len(batch)), map[string]interface{}{"reason": "producer_cancelled"})
		log.Warnf("context cancelled while producing log batch; dropped %d log record(s)", len(batch))
		return false
	}

	metricRecorder.Count(metrics.TierAdvanced, metrics.MetricBatchesCreated, 1, nil)
	metricRecorder.Summary(metrics.TierAdvanced, metrics.MetricBatchSizeBytes, float64(batchSize), nil)
	return true
}
