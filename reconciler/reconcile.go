package main

import (
	"context"
	"fmt"
)

// maxLogSourcesPerConnector is the OCI Service Connector Hub limit on the number
// of log-sources a single connector may reference. When a tenancy approaches this
// limit the design calls for sharding across multiple connectors; the reconciler
// refuses to exceed it rather than silently dropping a source. Confirm the exact
// value against current OCI SCH service limits before relying on it.
const maxLogSourcesPerConnector = 100

// reconcileResult summarizes what a single reconcile pass did, for logging.
type reconcileResult struct {
	Action    action
	Changed   bool
	LogGroup  string
	Before    int
	After     int
}

// reconcile brings the connector's log-source set into agreement with a single
// log-group lifecycle event. It is idempotent: adding a group already present or
// removing one already absent is a no-op that makes no SCH call. This keeps the
// reconciler safe under duplicate/retried event delivery (OCI Events is
// at-least-once).
func reconcile(ctx context.Context, mgr ConnectorManager, connectorID string, evt CloudEvent) (reconcileResult, error) {
	act := classify(evt.EventType)
	res := reconcileResult{Action: act, LogGroup: evt.Data.ResourceID}
	if act == actionUnknown {
		return res, fmt.Errorf("unhandled event type %q", evt.EventType)
	}
	if err := evt.validate(); err != nil {
		return res, err
	}

	current, err := mgr.ListLogSources(ctx, connectorID)
	if err != nil {
		return res, err
	}
	res.Before = len(current)

	desired, changed := applyAction(current, act, LogSourceRef{
		CompartmentID: evt.Data.CompartmentID,
		LogGroupID:    evt.Data.ResourceID,
	})
	res.After = len(desired)
	res.Changed = changed

	if !changed {
		// Idempotent no-op — nothing to write.
		return res, nil
	}

	if len(desired) > maxLogSourcesPerConnector {
		return res, fmt.Errorf("connector %s would exceed the %d log-source limit (has %d, adding %s); shard across connectors",
			connectorID, maxLogSourcesPerConnector, len(current), evt.Data.ResourceID)
	}

	if err := mgr.SetLogSources(ctx, connectorID, desired); err != nil {
		return res, err
	}
	return res, nil
}

// applyAction computes the desired log-source set from the current one. It
// returns the new slice and whether it differs from the input. Membership is
// keyed on log-group OCID, which is globally unique, so compartment mismatches
// on the same group are treated as the same entry (the incoming compartment wins
// on add).
func applyAction(current []LogSourceRef, act action, target LogSourceRef) ([]LogSourceRef, bool) {
	idx := -1
	for i, s := range current {
		if s.LogGroupID == target.LogGroupID {
			idx = i
			break
		}
	}

	switch act {
	case actionAdd:
		if idx >= 0 {
			return current, false // already present
		}
		return append(cloneRefs(current), target), true
	case actionRemove:
		if idx < 0 {
			return current, false // already absent
		}
		out := cloneRefs(current)
		return append(out[:idx], out[idx+1:]...), true
	default:
		return current, false
	}
}

// cloneRefs returns a copy so callers never mutate the slice returned by the SDK.
func cloneRefs(in []LogSourceRef) []LogSourceRef {
	out := make([]LogSourceRef, len(in))
	copy(out, in)
	return out
}
