// Package util provides generic utility functions.
package util

import (
	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// ProduceMessageToChannel sends a log batch to a channel for further processing.
func ProduceMessageToChannel(channel chan common.DetailedLogsBatch, currentBatch common.LogData, attributes common.LogAttributes) {
	channel <- []common.DetailedLog{{
		CommonData: common.Common{
			Attributes: attributes,
		},
		Entries: currentBatch,
	}}
}
