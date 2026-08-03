// Package main implements an Oracle Cloud Infrastructure (OCI) Function that keeps
// a Service Connector Hub (SCH) connector's log-source list in sync with the log
// groups that exist in a tenancy. It is triggered by the OCI Events service on
// log-group lifecycle events (create/delete) and reconciles the connector by
// calling UpdateServiceConnector, so newly created log groups are auto-subscribed
// for forwarding to New Relic and deleted ones are removed.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// action classifies what a log-group lifecycle event requires the reconciler to do.
type action int

const (
	// actionUnknown means the event does not map to a log-group lifecycle change we handle.
	actionUnknown action = iota
	// actionAdd means the referenced log group should be present in the connector.
	actionAdd
	// actionRemove means the referenced log group should be absent from the connector.
	actionRemove
)

func (a action) String() string {
	switch a {
	case actionAdd:
		return "add"
	case actionRemove:
		return "remove"
	default:
		return "unknown"
	}
}

// CloudEvent is the subset of the OCI Events (CloudEvents 0.1) envelope that the
// reconciler needs. OCI delivers this JSON as the function payload when an Events
// rule matches a log-group lifecycle event emitted by the loggingService source.
type CloudEvent struct {
	EventType string        `json:"eventType"`
	Source    string        `json:"source"`
	EventTime string        `json:"eventTime"`
	Data      CloudEventData `json:"data"`
}

// CloudEventData carries the resource identity for the event.
type CloudEventData struct {
	CompartmentID   string `json:"compartmentId"`
	CompartmentName string `json:"compartmentName"`
	ResourceName    string `json:"resourceName"`
	// ResourceID is the OCID of the log group that was created/deleted.
	ResourceID string `json:"resourceId"`
}

// parseEvent reads and decodes the CloudEvent payload delivered by OCI Events.
func parseEvent(in io.Reader) (CloudEvent, error) {
	var evt CloudEvent
	body, err := io.ReadAll(in)
	if err != nil {
		return evt, fmt.Errorf("failed to read event body: %w", err)
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return evt, fmt.Errorf("failed to unmarshal cloud event: %w", err)
	}
	return evt, nil
}

// classify maps an event type string to the reconciler action. Matching is done
// on a lower-cased suffix so it is tolerant of the exact namespace OCI uses
// (e.g. "com.oraclecloud.loggingservice.createloggroup"). The exact strings
// should be pinned in the Events rule once observed in the target tenancy.
func classify(eventType string) action {
	et := strings.ToLower(eventType)
	switch {
	case strings.HasSuffix(et, "createloggroup"):
		return actionAdd
	case strings.HasSuffix(et, "deleteloggroup"):
		return actionRemove
	// An update is treated as an upsert: ensure the group is present. This is
	// harmless because add is idempotent and keeps the connector converged even
	// if a create event was missed.
	case strings.HasSuffix(et, "updateloggroup"):
		return actionAdd
	default:
		return actionUnknown
	}
}

// validate ensures the event carries the identity the reconciler needs to act.
func (e CloudEvent) validate() error {
	if e.Data.ResourceID == "" {
		return fmt.Errorf("event %q has empty data.resourceId (log group OCID)", e.EventType)
	}
	if e.Data.CompartmentID == "" {
		return fmt.Errorf("event %q has empty data.compartmentId", e.EventType)
	}
	return nil
}
