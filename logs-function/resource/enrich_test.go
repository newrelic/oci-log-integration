package resource

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResolver is a test double for Resolver. It records exactly what OCIDs it was asked to
// resolve (so tests can assert on dedup behavior) and returns whatever resolved/err were
// configured.
type fakeResolver struct {
	callCount     int
	ocidsReceived []string
	resolved      map[string]string
	err           error
}

func (f *fakeResolver) ResolveMany(ctx context.Context, ocids []string) (map[string]string, error) {
	f.callCount++
	f.ocidsReceived = append([]string{}, ocids...)
	return f.resolved, f.err
}

// firewallRecord builds a record matching the still-active Network Firewall rule -- used as a
// generic "needs resolve" fixture for EnrichRecords tests that aren't about Network Firewall
// itself. VCN flow logs and Bastion used to serve this role, but both were dropped from rules.go
// (no confirmed entity-definitions rule for either), so this package no longer has any way to
// build a matched record via a real rule from those two.
func firewallRecord(firewallOCID string) map[string]interface{} {
	return map[string]interface{}{
		"type": "com.oraclecloud.networkfirewall.traffic",
		"data": map[string]interface{}{"firewall-id": firewallOCID},
	}
}

func unmatchedVaultRecord() map[string]interface{} {
	return map[string]interface{}{
		"type": "com.oraclecloud.KeyManagementService.GetVault",
		"data": map[string]interface{}{
			"resourceId": "ocid1.vault.oc1.iad.ejurnwh5aabpq.abuwcljsa2gl3wxoj2ftbtwijb64qumg5dj3zbor6oasactkqqcngcjoszra",
		},
	}
}

func dataOf(t *testing.T, rec map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, _ := rec["data"].(map[string]interface{})
	return data
}

func TestEnrichRecords_DedupesBeforeResolve(t *testing.T) {
	const firewallOCID = "ocid1.networkfirewall.oc1.iad.a"
	records := common.OCILoggingEvent{firewallRecord(firewallOCID), firewallRecord(firewallOCID), firewallRecord(firewallOCID)}
	resolver := &fakeResolver{resolved: map[string]string{firewallOCID: "prod-firewall"}}

	EnrichRecords(context.Background(), records, resolver)

	assert.Equal(t, 1, resolver.callCount, "ResolveMany should be called exactly once per invocation")
	assert.Equal(t, []string{firewallOCID}, resolver.ocidsReceived, "three records with the same OCID should dedupe to one entry")
}

func TestEnrichRecords_InjectsResolvedName(t *testing.T) {
	const firewallOCID = "ocid1.networkfirewall.oc1.iad.a"
	rec := firewallRecord(firewallOCID)
	records := common.OCILoggingEvent{rec}
	resolver := &fakeResolver{resolved: map[string]string{firewallOCID: "prod-firewall"}}

	EnrichRecords(context.Background(), records, resolver)

	assert.Equal(t, "prod-firewall", dataOf(t, rec)["logging.oci.displayName"])
}

func TestEnrichRecords_PartialResolveFailureStillInjectsWhatResolved(t *testing.T) {
	firewallRec := map[string]interface{}{
		"type": "com.oraclecloud.networkfirewall.traffic",
		"data": map[string]interface{}{"firewall-id": "ocid1.networkfirewall.oc1.iad.a"},
	}
	dnsRec := map[string]interface{}{
		"type":   "com.oraclecloud.dns.private.resolver",
		"source": "ocid1.dnsresolver.oc1.iad.b",
	}
	records := common.OCILoggingEvent{firewallRec, dnsRec}
	// Only the firewall OCID resolves; the resolver also reports an error (e.g. one chunk of a
	// multi-chunk call failed) -- resolution should still be treated as best-effort, not fatal.
	resolver := &fakeResolver{
		resolved: map[string]string{"ocid1.networkfirewall.oc1.iad.a": "prod-firewall"},
		err:      errors.New("chunk failed"),
	}

	assert.NotPanics(t, func() {
		EnrichRecords(context.Background(), records, resolver)
	})

	assert.Equal(t, "prod-firewall", dataOf(t, firewallRec)["logging.oci.displayName"])

	assert.NotContains(t, dataOf(t, dnsRec), "logging.oci.displayName", "unresolved OCID should not get a name")
}

func TestEnrichRecords_NilResolverNeverInjectsAName(t *testing.T) {
	rec := firewallRecord("ocid1.networkfirewall.oc1.iad.c")
	records := common.OCILoggingEvent{rec}

	assert.NotPanics(t, func() {
		EnrichRecords(context.Background(), records, nil)
	})

	assert.NotContains(t, dataOf(t, rec), "logging.oci.displayName", "no resolver was available, so this can't have gotten a name")
}

func TestEnrichRecords_UnmatchedRecordGetsNoInjectionAtAll(t *testing.T) {
	rec := unmatchedVaultRecord()
	records := common.OCILoggingEvent{rec}
	resolver := &fakeResolver{}

	EnrichRecords(context.Background(), records, resolver)

	assert.Zero(t, resolver.callCount)
	data := dataOf(t, rec)
	assert.NotContains(t, data, "logging.oci.displayName", "an unmatched record should gain no new keys at all")
	assert.Len(t, data, 1, "the record's own pre-existing data map (just resourceId) should be untouched, not added to")
}

func TestEnrichRecords_RecordWithNoDataMapNeverGetsOneCreated(t *testing.T) {
	rec := map[string]interface{}{
		"type": "com.oraclecloud.KeyManagementService.GetVault",
		// deliberately no "data" key at all, unlike unmatchedVaultRecord()
	}
	records := common.OCILoggingEvent{rec}

	EnrichRecords(context.Background(), records, &fakeResolver{})

	_, dataExists := rec["data"]
	assert.False(t, dataExists, "injectResourceName's early return means a data map is never created when there's nothing to write")
}

func TestResolveTimeout(t *testing.T) {
	defer os.Unsetenv(common.ResourceResolveTimeoutSeconds)

	os.Unsetenv(common.ResourceResolveTimeoutSeconds)
	assert.Equal(t, 10, int(resolveTimeout().Seconds()), "default should be DefaultResourceResolveTimeoutSeconds")

	os.Setenv(common.ResourceResolveTimeoutSeconds, "30")
	assert.Equal(t, 30, int(resolveTimeout().Seconds()), "a valid override should be honored")

	os.Setenv(common.ResourceResolveTimeoutSeconds, "not-a-number")
	assert.Equal(t, 10, int(resolveTimeout().Seconds()), "an invalid override should fall back to the default")

	os.Setenv(common.ResourceResolveTimeoutSeconds, "-5")
	assert.Equal(t, 10, int(resolveTimeout().Seconds()), "a non-positive override should fall back to the default")
}

// Real-sample coverage: run every real sample from extract_test.go through EnrichRecords (not
// just Extract), both individually and combined into one mixed multi-record batch -- this is what
// actually proves the injected logging.oci.displayName field lands correctly, which Extract-level
// testing alone can't show.

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

// TestEnrichRecords_RealSamples_CombinedBatch takes every remaining real sample that matches a
// rule, combines them into a single JSON array and unmarshals them together into one
// common.OCILoggingEvent -- the same shape a real multi-record OCI Function invocation payload
// takes -- then runs them through EnrichRecords in exactly one call. This proves every individual
// record still ends up correct even mixed together with unrelated types in one call (the
// cross-record dedup itself is exercised more directly by TestEnrichRecords_DedupesBeforeResolve).
// VCN flow logs and Bastion are excluded here -- no rule in rules.go for either (no confirmed
// entity-definitions rule), so they'd add nothing to a test about matched records resolving
// correctly; their unmatched behavior is already covered by TestEnrichRecords_RealSamples above.
func TestEnrichRecords_RealSamples_CombinedBatch(t *testing.T) {
	samples := []string{
		sampleNetworkFirewallTraffic,
		sampleDNSPrivateResolver,
		sampleIntegrationActivityStream,
		sampleGoldenGate,
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

	// All 5 records (Network Firewall, DNS Private Resolver, Integration Cloud, GoldenGate,
	// Events rule-execution) match a rule and need a resolve -- exactly one call, exactly 5
	// distinct OCIDs, across the entire batch.
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

			require.NotEmpty(t, ocid, "record %d: every sample in this batch matches a rule", i)
			want, ok := wantNameForOCID[ocid]
			require.True(t, ok, "record %d: unexpected OCID %s not accounted for in this test", i, ocid)
			assert.Equal(t, want, data["logging.oci.displayName"], "record %d (type=%s, ocid=%s)", i, logType, ocid)
		})
	}
}
