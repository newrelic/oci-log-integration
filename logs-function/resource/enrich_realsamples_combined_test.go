package resource

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Step 4: every remaining real sample combined into a single JSON array and unmarshaled
// together into one common.OCILoggingEvent -- the same shape a real multi-record OCI Function
// invocation payload takes -- then run through EnrichRecords in exactly one call. This proves
// every individual record still ends up correct even mixed together with unrelated types in one
// call, matched and unmatched alike (the cross-record dedup itself is exercised more directly by
// TestEnrichRecords_DedupesBeforeResolve). VCN flow logs (NLB Connection Log, VNIC ACCEPT/REJECT)
// and Bastion samples are still included here specifically to prove they stay unmatched even
// inside a mixed batch, now that both were dropped from rules.go (no confirmed
// entity-definitions rule for either).
func TestEnrichRecords_RealSamples_CombinedBatch(t *testing.T) {
	samples := []string{
		sampleNLBConnectionLog,
		sampleNetworkFirewallTraffic,
		sampleVNICAccept,
		sampleVNICReject,
		sampleDNSPrivateResolver,
		sampleIntegrationActivityStream,
		sampleGoldenGate,
		sampleBastionListSessions,
		sampleBastionGetBastion,
		sampleEventsRuleExecutionLog,
	}
	combined := "[" + strings.Join(samples, ",") + "]"

	var records common.OCILoggingEvent
	require.NoError(t, json.Unmarshal([]byte(combined), &records), "combined batch must be valid JSON")
	require.Len(t, records, len(samples), "every sample should have unmarshaled into its own record")

	const (
		firewallOCID = "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq"
		dnsOCID      = "ocid1.dnsresolver.oc1.iad.amaaaaaatvlqdbyaf4mgekhmbsvoxvyluwzhowzpdci4rzjrgat6n7z2yj4q"
		integOCID    = "ocid1.integrationinstance.oc1.iad.amaaaaaatvlqdbyassm3eanvviwuoijb57xkalojoiovyfuu6lslkmx7b6lq"
		ggOCID       = "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca"
		ruleOCID     = "ocid1.eventrule.oc1.iad.amaaaaaatvlqdbyacot7p5fphd6pbcuz3zt2x7jgc4fl7xxqlrhqmoz6gjfq"
	)

	resolver := &fakeResolver{resolved: map[string]string{
		firewallOCID: "resolved-firewall-name",
		dnsOCID:      "resolved-dns-resolver-name",
		integOCID:    "resolved-integration-name",
		ggOCID:       "resolved-goldengate-name",
		ruleOCID:     "resolved-event-rule-name",
	}}

	EnrichRecords(context.Background(), records, resolver)

	// Of the 10 records, only the 5 matched types (Network Firewall, DNS Private Resolver,
	// Integration Cloud, GoldenGate, Events rule-execution) need a resolve; the VCN flow log and
	// Bastion samples are unmatched and never reach the resolver at all -- exactly one call,
	// exactly 5, across the entire batch.
	assert.Equal(t, 1, resolver.callCount, "ResolveMany should be called exactly once for the whole batch")
	assert.ElementsMatch(t,
		[]string{firewallOCID, dnsOCID, integOCID, ggOCID, ruleOCID},
		resolver.ocidsReceived,
		"exactly the 5 distinct OCIDs needing resolution, deduped across the whole batch")

	// Spot-check every individual record still ended up correct, even mixed into one batch.
	wantNameForOCID := map[string]string{
		firewallOCID: "resolved-firewall-name",
		dnsOCID:      "resolved-dns-resolver-name",
		integOCID:    "resolved-integration-name",
		ggOCID:       "resolved-goldengate-name",
		ruleOCID:     "resolved-event-rule-name",
	}

	for i, rec := range records {
		logType, _ := rec["type"].(string)
		data := dataOf(t, rec)
		// re-derive the OCID from the original payload fields (rules.go's OCIDPath) rather than
		// from an injected field, since the name-only injection no longer records the OCID itself.
		ocid := Extract(rec).OCID

		t.Run(logType, func(t *testing.T) {
			if pretty, err := json.MarshalIndent(rec, "", "  "); err == nil {
				t.Logf("record %d full payload after enrichment:\n%s", i, pretty)
			}

			if ocid == "" {
				// VCN flow logs and Bastion are unmatched (no rule in rules.go for either), so
				// nothing to resolve and nothing to inject.
				assert.NotContains(t, data, "logging.oci.displayName", "record %d: no OCID means no name either", i)
				return
			}
			want, ok := wantNameForOCID[ocid]
			require.True(t, ok, "record %d: unexpected OCID %s not accounted for in this test", i, ocid)
			assert.Equal(t, want, data["logging.oci.displayName"], "record %d (type=%s, ocid=%s)", i, logType, ocid)
		})
	}
}
