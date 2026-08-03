package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeManager is an in-memory ConnectorManager for exercising reconcile logic
// without the OCI SDK. It records how many times SetLogSources was called so
// tests can assert idempotency (no write on no-op).
type fakeManager struct {
	sources  []LogSourceRef
	listErr  error
	setErr   error
	setCalls int
}

func (f *fakeManager) ListLogSources(_ context.Context, _ string) ([]LogSourceRef, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.sources, nil
}

func (f *fakeManager) SetLogSources(_ context.Context, _ string, sources []LogSourceRef) error {
	f.setCalls++
	if f.setErr != nil {
		return f.setErr
	}
	f.sources = sources
	return nil
}

func createEvent(compartment, logGroup string) CloudEvent {
	return CloudEvent{
		EventType: "com.oraclecloud.loggingservice.createloggroup",
		Data:      CloudEventData{CompartmentID: compartment, ResourceID: logGroup},
	}
}

func deleteEvent(compartment, logGroup string) CloudEvent {
	return CloudEvent{
		EventType: "com.oraclecloud.loggingservice.deleteloggroup",
		Data:      CloudEventData{CompartmentID: compartment, ResourceID: logGroup},
	}
}

func TestReconcile_AddNewLogGroup(t *testing.T) {
	mgr := &fakeManager{sources: []LogSourceRef{{CompartmentID: "c1", LogGroupID: "lg-a"}}}

	res, err := reconcile(context.Background(), mgr, "conn-1", createEvent("c1", "lg-b"))

	require.NoError(t, err)
	assert.True(t, res.Changed)
	assert.Equal(t, 1, res.Before)
	assert.Equal(t, 2, res.After)
	assert.Equal(t, 1, mgr.setCalls)
	assert.Contains(t, mgr.sources, LogSourceRef{CompartmentID: "c1", LogGroupID: "lg-b"})
}

// #9 in the design tracker: adding a log group that is already subscribed must
// not duplicate it and must not issue a write.
func TestReconcile_AddDuplicateIsNoOp(t *testing.T) {
	mgr := &fakeManager{sources: []LogSourceRef{{CompartmentID: "c1", LogGroupID: "lg-a"}}}

	res, err := reconcile(context.Background(), mgr, "conn-1", createEvent("c1", "lg-a"))

	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.Equal(t, 0, mgr.setCalls, "no SCH write should occur for a duplicate add")
	assert.Len(t, mgr.sources, 1)
}

// #7 in the design tracker: deleting a log group removes exactly that source.
func TestReconcile_RemoveLogGroup(t *testing.T) {
	mgr := &fakeManager{sources: []LogSourceRef{
		{CompartmentID: "c1", LogGroupID: "lg-a"},
		{CompartmentID: "c1", LogGroupID: "lg-b"},
	}}

	res, err := reconcile(context.Background(), mgr, "conn-1", deleteEvent("c1", "lg-a"))

	require.NoError(t, err)
	assert.True(t, res.Changed)
	assert.Equal(t, 2, res.Before)
	assert.Equal(t, 1, res.After)
	assert.Equal(t, []LogSourceRef{{CompartmentID: "c1", LogGroupID: "lg-b"}}, mgr.sources)
}

func TestReconcile_RemoveAbsentIsNoOp(t *testing.T) {
	mgr := &fakeManager{sources: []LogSourceRef{{CompartmentID: "c1", LogGroupID: "lg-a"}}}

	res, err := reconcile(context.Background(), mgr, "conn-1", deleteEvent("c1", "lg-x"))

	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.Equal(t, 0, mgr.setCalls)
}

// #8 in the design tracker: the connector must never exceed the SCH log-source
// limit; the reconciler errors instead of writing an over-limit set.
func TestReconcile_RefusesToExceedLimit(t *testing.T) {
	sources := make([]LogSourceRef, maxLogSourcesPerConnector)
	for i := range sources {
		sources[i] = LogSourceRef{CompartmentID: "c1", LogGroupID: "lg-" + string(rune('A'+i%26)) + string(rune('0'+i/26))}
	}
	mgr := &fakeManager{sources: sources}

	_, err := reconcile(context.Background(), mgr, "conn-1", createEvent("c1", "lg-overflow"))

	require.Error(t, err)
	assert.Equal(t, 0, mgr.setCalls, "must not write an over-limit source set")
}

func TestReconcile_UnknownEventErrors(t *testing.T) {
	mgr := &fakeManager{}
	evt := CloudEvent{EventType: "com.oraclecloud.loggingservice.createlog", Data: CloudEventData{CompartmentID: "c1", ResourceID: "lg-a"}}

	_, err := reconcile(context.Background(), mgr, "conn-1", evt)

	require.Error(t, err)
	assert.Equal(t, 0, mgr.setCalls)
}

func TestReconcile_PropagatesListError(t *testing.T) {
	mgr := &fakeManager{listErr: errors.New("boom")}

	_, err := reconcile(context.Background(), mgr, "conn-1", createEvent("c1", "lg-a"))

	require.Error(t, err)
}
