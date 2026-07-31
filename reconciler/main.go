// POC reconciler: keep a Service Connector's logging sources in sync with the
// tenancy's log groups. An OCI Events rule fires on log-group create/delete and
// invokes this function, which adds or removes the matching log source on the
// connector. Bare-minimum logic only — no retries, tests, or abstractions.
//
// SDK type/field names (sch.LoggingSourceDetails, sch.LogSource, etc.) should be
// confirmed with `go vet` in a real toolchain.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	fdk "github.com/fnproject/fdk-go"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/sch"
)

type cloudEvent struct {
	EventType string `json:"eventType"`
	Data      struct {
		CompartmentId string `json:"compartmentId"`
		ResourceId    string `json:"resourceId"` // log group OCID
	} `json:"data"`
}

func main() {
	fdk.Handle(fdk.HandlerFunc(handle))
}

func handle(ctx context.Context, in io.Reader, _ io.Writer) {
	var evt cloudEvent
	if err := json.NewDecoder(in).Decode(&evt); err != nil {
		panic(fmt.Errorf("decode event: %w", err))
	}

	et := strings.ToLower(evt.EventType)
	add := strings.Contains(et, "createloggroup")
	remove := strings.Contains(et, "deleteloggroup")
	if !add && !remove {
		return // not a log-group lifecycle event
	}

	connectorID := os.Getenv("SERVICE_CONNECTOR_OCID")

	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		panic(fmt.Errorf("resource principal: %w", err))
	}
	client, err := sch.NewConnectorClientWithConfigurationProvider(provider)
	if err != nil {
		panic(fmt.Errorf("sch client: %w", err))
	}

	got, err := client.GetServiceConnector(ctx, sch.GetServiceConnectorRequest{ServiceConnectorId: &connectorID})
	if err != nil {
		panic(fmt.Errorf("get connector: %w", err))
	}
	src, ok := got.ServiceConnector.Source.(sch.LoggingSourceDetails)
	if !ok {
		panic("connector source is not a logging source")
	}

	src.LogSources = mutate(src.LogSources, evt.Data.CompartmentId, evt.Data.ResourceId, add)

	_, err = client.UpdateServiceConnector(ctx, sch.UpdateServiceConnectorRequest{
		ServiceConnectorId: &connectorID,
		UpdateServiceConnectorDetails: sch.UpdateServiceConnectorDetails{
			Source: sch.UpdateLoggingSourceDetails{LogSources: src.LogSources},
		},
	})
	if err != nil {
		panic(fmt.Errorf("update connector: %w", err))
	}
}

// mutate adds or removes the log group source, idempotently.
func mutate(sources []sch.LogSource, compartmentID, logGroupID string, add bool) []sch.LogSource {
	out := make([]sch.LogSource, 0, len(sources)+1)
	found := false
	for _, s := range sources {
		if s.LogGroupId != nil && *s.LogGroupId == logGroupID {
			found = true
			if !add {
				continue // removing: drop it
			}
		}
		out = append(out, s)
	}
	if add && !found {
		out = append(out, sch.LogSource{CompartmentId: &compartmentID, LogGroupId: &logGroupID})
	}
	return out
}
