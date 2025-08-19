// Package main implements an Oracle Cloud Infrastructure (OCI) Function that processes
// OCI Logging events and forwards them to New Relic's logging platform. The function
// handles event unmarshaling, batching, and concurrent processing for optimal performance.
package main

import (
	"context"
	"io"
	"sync"

	"github.com/fnproject/fdk-go"
	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/newrelic/oci-log-integration/logs-function/loggroup"
	"github.com/newrelic/oci-log-integration/logs-function/unmarshal"
	"github.com/newrelic/oci-log-integration/logs-function/util"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

func main() {
	log.Debug("Initializing NewRelic client (cached for container lifecycle)")
	nrClient, err := util.NewNRClient()
	if err != nil {
		log.Fatalf("error initializing newrelic client: %v", err)
	} else {
		log.Debug("NewRelic client initialized successfully")
		handler := func(ctx context.Context, in io.Reader, out io.Writer) {
			handleFunctionWithClient(ctx, in, out, nrClient)
		}
		fdk.Handle(fdk.HandlerFunc(handler))
	}
}

// handleFunctionWithClient processes OCI logging events and forwards them to New Relic.
// It unmarshals incoming events, starts worker goroutines to process log batches concurrently,
// and waits for all processing to complete before returning.
func handleFunctionWithClient(ctx context.Context, in io.Reader, _ io.Writer, nrClient util.NewRelicClientAPI) {
	event := unmarshal.Event{}
	if err := event.Unmarshal(in); err != nil {
		log.Errorf("Error unmarshalling event: %v", err)
		return
	}

	channel := make(chan common.DetailedLogsBatch)
	var wg sync.WaitGroup
	wg.Add(common.NumberOfWorkers)

	// Start multiple worker goroutines to process log batches concurrently
	for i := 0; i < common.NumberOfWorkers; i++ {
		go util.ConsumeLogBatches(ctx, channel, &wg, nrClient)
	}

	switch event.EventType {
	case unmarshal.OCI_LOGGING:
		loggroup.ProcessLogs(event.OCILoggingEvent, channel)
	default:
		log.Warnf("Unknown event type: %s", event.EventType)
	}

	// Close channel after processing to signal completion
	close(channel)
	// Wait for goroutines to finish processing
	wg.Wait()
}
