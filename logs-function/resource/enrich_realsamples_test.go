package resource

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
)

// Step 3: run every real sample from steps 1 and 2 through EnrichRecords (not just Extract),
// with a fake resolver configured to return a known name for each OCID that genuinely needs
// resolving -- this is what actually proves the injected logging.oci.displayName field lands
// correctly, which Extract-level testing alone can't show.
func TestEnrichRecords_RealSamples(t *testing.T) {
	const (
		firewallOCID = "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq"
		dnsOCID      = "ocid1.dnsresolver.oc1.iad.amaaaaaatvlqdbyaf4mgekhmbsvoxvyluwzhowzpdci4rzjrgat6n7z2yj4q"
		integOCID    = "ocid1.integrationinstance.oc1.iad.amaaaaaatvlqdbyassm3eanvviwuoijb57xkalojoiovyfuu6lslkmx7b6lq"
		ggOCID       = "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca"
		ruleOCID     = "ocid1.eventrule.oc1.iad.amaaaaaatvlqdbyacot7p5fphd6pbcuz3zt2x7jgc4fl7xxqlrhqmoz6gjfq"
	)

	// One shared resolver, configured with a distinct, made-up name per OCID that genuinely
	// needs resolving -- distinct on purpose (e.g. "resolved-event-rule-name" rather than
	// "poc-reconciler-rule") so a test passing can't be confused with the name having leaked in
	// from source/somewhere else instead of actually coming from the resolver.
	resolver := &fakeResolver{resolved: map[string]string{
		firewallOCID: "resolved-firewall-name",
		dnsOCID:      "resolved-dns-resolver-name",
		integOCID:    "resolved-integration-name",
		ggOCID:       "resolved-goldengate-name",
		ruleOCID:     "resolved-event-rule-name",
	}}

	tests := []struct {
		name     string
		raw      string
		wantName string // expected logging.oci.displayName; "" means the key should be absent
	}{
		{name: "NLB Connection Log (VCN flow logs) -- unmatched, nothing to inject", raw: sampleNLBConnectionLog},
		{name: "Network Firewall traffic -- resolved", raw: sampleNetworkFirewallTraffic, wantName: "resolved-firewall-name"},
		{name: "VNIC ACCEPT -- unmatched, nothing to inject", raw: sampleVNICAccept},
		{name: "VNIC REJECT -- unmatched, nothing to inject", raw: sampleVNICReject},
		{name: "DNS Private Resolver -- resolved", raw: sampleDNSPrivateResolver, wantName: "resolved-dns-resolver-name"},
		{name: "Integration activity stream -- resolved", raw: sampleIntegrationActivityStream, wantName: "resolved-integration-name"},
		{name: "GoldenGate -- resolved", raw: sampleGoldenGate, wantName: "resolved-goldengate-name"},
		{name: "Bastion ListSessions -- unmatched, nothing to inject", raw: sampleBastionListSessions},
		{name: "Bastion GetBastion -- unmatched, nothing to inject", raw: sampleBastionGetBastion},
		{name: "Events rule-execution log -- resolved", raw: sampleEventsRuleExecutionLog, wantName: "resolved-event-rule-name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := parseSample(t, tt.raw)
			records := common.OCILoggingEvent{record}

			EnrichRecords(context.Background(), records, resolver)

			data := dataOf(t, record)
			if tt.wantName == "" {
				assert.NotContains(t, data, "logging.oci.displayName")
			} else {
				assert.Equal(t, tt.wantName, data["logging.oci.displayName"])
			}
			pretty, err := json.MarshalIndent(record, "", "  ")
			if err == nil {
				t.Logf("full payload after enrichment:\n%s", pretty)
			}
		})
	}
}
