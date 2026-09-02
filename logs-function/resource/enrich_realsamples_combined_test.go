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
// two things steps 1-3 couldn't on their own: dedup collapses NLB Connection Log and VNIC
// REJECT's shared vnicocid into a single resolve request across the whole batch (not just
// within one type), and every individual record still ends up correct even mixed together with
// unrelated types in one call.
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
		nlbVnicOCID  = "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq"
		acceptVnic   = "ocid1.vnic.oc1.iad.abuwcljtsxsa7nu6spexecas25aw33gqufj2ljccwocl34xqcl6hhstcclta"
		firewallOCID = "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq"
		dnsOCID      = "ocid1.dnsresolver.oc1.iad.amaaaaaatvlqdbyaf4mgekhmbsvoxvyluwzhowzpdci4rzjrgat6n7z2yj4q"
		integOCID    = "ocid1.integrationinstance.oc1.iad.amaaaaaatvlqdbyassm3eanvviwuoijb57xkalojoiovyfuu6lslkmx7b6lq"
		ggOCID       = "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca"
		bastionOCID  = "ocid1.bastion.oc1.iad.amaaaaaatvlqdbyagt36dlcwb6zdma3ddbix74hdcge5xvfnewy6heaovyjq"
		ruleOCID     = "ocid1.eventrule.oc1.iad.amaaaaaatvlqdbyacot7p5fphd6pbcuz3zt2x7jgc4fl7xxqlrhqmoz6gjfq"
	)

	resolver := &fakeResolver{resolved: map[string]string{
		nlbVnicOCID:  "resolved-nlb-vnic-name",
		acceptVnic:   "resolved-accept-vnic-name",
		firewallOCID: "resolved-firewall-name",
		dnsOCID:      "resolved-dns-resolver-name",
		integOCID:    "resolved-integration-name",
		ggOCID:       "resolved-goldengate-name",
		ruleOCID:     "resolved-event-rule-name",
	}}

	EnrichRecords(context.Background(), records, resolver)

	// The whole point of the dedup: 8 of the 10 records need a resolve, but only 7 distinct
	// OCIDs among them (NLB Connection Log and VNIC REJECT share nlbVnicOCID) -- exactly one
	// call, exactly 7, across the entire batch, not per record and not per type.
	assert.Equal(t, 1, resolver.callCount, "ResolveMany should be called exactly once for the whole batch")
	assert.ElementsMatch(t,
		[]string{nlbVnicOCID, acceptVnic, firewallOCID, dnsOCID, integOCID, ggOCID, ruleOCID},
		resolver.ocidsReceived,
		"exactly the 7 distinct OCIDs needing resolution, deduped across the whole batch")

	// Spot-check every individual record still ended up correct, even mixed into one batch.
	wantNameForOCID := map[string]string{
		nlbVnicOCID:  "resolved-nlb-vnic-name",
		acceptVnic:   "resolved-accept-vnic-name",
		firewallOCID: "resolved-firewall-name",
		dnsOCID:      "resolved-dns-resolver-name",
		integOCID:    "resolved-integration-name",
		ggOCID:       "resolved-goldengate-name",
		ruleOCID:     "resolved-event-rule-name",
		bastionOCID:  "demo-bastion",
	}

	for i, rec := range records {
		logContent := logContentOf(t, rec)
		logType, _ := logContent["type"].(string)
		data := dataOf(t, rec)
		// re-derive the OCID from the original payload fields (rules.go's OCIDPath) rather than
		// from an injected field, since the name-only injection no longer records the OCID itself.
		ocid := Extract(rec).OCID

		t.Run(logType, func(t *testing.T) {
			if pretty, err := json.MarshalIndent(rec, "", "  "); err == nil {
				t.Logf("record %d full payload after enrichment:\n%s", i, pretty)
			}

			if ocid == "" {
				// Bastion ListSessions -- matched the rule, but neither field was present, so
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
