package main

import (
	"context"
	"fmt"

	ociCommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/sch"
)

// LogSourceRef is a local, SDK-agnostic representation of a single log-source
// entry on a Service Connector. Keeping the reconcile logic in terms of this
// type (rather than sch.LogSource) makes it unit-testable without the SDK.
type LogSourceRef struct {
	CompartmentID string
	LogGroupID    string
}

// ConnectorManager is the narrow surface the reconciler needs from SCH. It is an
// interface so tests can substitute a fake and exercise the reconcile logic
// without a live tenancy. The concrete implementation is schConnectorManager.
type ConnectorManager interface {
	// ListLogSources returns the current logging log-sources on the connector.
	ListLogSources(ctx context.Context, connectorID string) ([]LogSourceRef, error)
	// SetLogSources replaces the connector's logging log-sources with the given set.
	SetLogSources(ctx context.Context, connectorID string, sources []LogSourceRef) error
}

// schConnectorManager implements ConnectorManager over the OCI SCH SDK client.
type schConnectorManager struct {
	client schClient
}

// schClient is the subset of sch.ConnectorClient used here, extracted as an
// interface for testability.
type schClient interface {
	GetServiceConnector(ctx context.Context, request sch.GetServiceConnectorRequest) (sch.GetServiceConnectorResponse, error)
	UpdateServiceConnector(ctx context.Context, request sch.UpdateServiceConnectorRequest) (sch.UpdateServiceConnectorResponse, error)
}

// newSCHConnectorManager builds a ConnectorManager backed by a real SCH client
// authenticated via Resource Principal (the auth mode used elsewhere in this
// repo — see logs-function/util/secrets_util.go). If region is non-empty the
// client is pinned to it; otherwise the Resource Principal's home region is used.
func newSCHConnectorManager(region string) (ConnectorManager, error) {
	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to create resource principal configuration provider: %w", err)
	}

	client, err := sch.NewServiceConnectorClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SCH connector client: %w", err)
	}
	if region != "" {
		client.SetRegion(region)
	}

	return &schConnectorManager{client: &client}, nil
}

// ListLogSources fetches the connector and returns its logging source entries.
func (m *schConnectorManager) ListLogSources(ctx context.Context, connectorID string) ([]LogSourceRef, error) {
	resp, err := m.client.GetServiceConnector(ctx, sch.GetServiceConnectorRequest{
		ServiceConnectorId: ociCommon.String(connectorID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get service connector %s: %w", connectorID, err)
	}

	loggingSource, ok := resp.ServiceConnector.Source.(sch.LoggingSourceDetailsResponse)
	if !ok {
		return nil, fmt.Errorf("connector %s source is not a logging source (kind=%T); reconciler only manages logging connectors", connectorID, resp.ServiceConnector.Source)
	}

	refs := make([]LogSourceRef, 0, len(loggingSource.LogSources))
	for _, ls := range loggingSource.LogSources {
		refs = append(refs, LogSourceRef{
			CompartmentID: strVal(ls.CompartmentId),
			LogGroupID:    strVal(ls.LogGroupId),
		})
	}
	return refs, nil
}

// strVal dereferences an OCI SDK *string, returning "" for nil.
func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// SetLogSources replaces the connector's logging log-sources with the given set.
func (m *schConnectorManager) SetLogSources(ctx context.Context, connectorID string, sources []LogSourceRef) error {
	logSources := make([]sch.LogSource, 0, len(sources))
	for _, s := range sources {
		s := s
		logSources = append(logSources, sch.LogSource{
			CompartmentId: ociCommon.String(s.CompartmentID),
			LogGroupId:    ociCommon.String(s.LogGroupID),
		})
	}

	_, err := m.client.UpdateServiceConnector(ctx, sch.UpdateServiceConnectorRequest{
		ServiceConnectorId: ociCommon.String(connectorID),
		UpdateServiceConnectorDetails: sch.UpdateServiceConnectorDetails{
			Source: sch.LoggingSourceDetails{
				LogSources: logSources,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update service connector %s: %w", connectorID, err)
	}
	return nil
}
