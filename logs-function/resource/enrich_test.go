package resource

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/stretchr/testify/assert"
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

func vnicRecord(vnicOCID string) map[string]interface{} {
	return map[string]interface{}{
		"logContent": map[string]interface{}{
			"type": "com.oraclecloud.vcn.flowlogs.DataEvent",
			"oracle": map[string]interface{}{
				"vnicocid": vnicOCID,
			},
			"source": "-",
		},
	}
}

func bastionGetBastionRecord() map[string]interface{} {
	return map[string]interface{}{
		"logContent": map[string]interface{}{
			"type":   "com.oraclecloud.bastion.GetBastion",
			"source": "demo-bastion",
			"data": map[string]interface{}{
				"resourceId": "ocid1.bastion.oc1.iad.amaaaaaatvlqdbyagt36dlcwb6zdma3ddbix74hdcge5xvfnewy6heaovyjq",
			},
		},
	}
}

func unmatchedVaultRecord() map[string]interface{} {
	return map[string]interface{}{
		"logContent": map[string]interface{}{
			"type": "com.oraclecloud.KeyManagementService.GetVault",
			"data": map[string]interface{}{
				"resourceId": "ocid1.vault.oc1.iad.ejurnwh5aabpq.abuwcljsa2gl3wxoj2ftbtwijb64qumg5dj3zbor6oasactkqqcngcjoszra",
			},
		},
	}
}

func dataOf(t *testing.T, rec map[string]interface{}) map[string]interface{} {
	t.Helper()
	logContent, ok := rec["logContent"].(map[string]interface{})
	if !ok {
		return nil
	}
	data, _ := logContent["data"].(map[string]interface{})
	return data
}

// logContentOf returns a record's logContent map.
func logContentOf(t *testing.T, rec map[string]interface{}) map[string]interface{} {
	t.Helper()
	logContent, _ := rec["logContent"].(map[string]interface{})
	return logContent
}

func TestEnrichRecords_DedupesBeforeResolve(t *testing.T) {
	const vnic = "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq"
	records := common.OCILoggingEvent{vnicRecord(vnic), vnicRecord(vnic), vnicRecord(vnic)}
	resolver := &fakeResolver{resolved: map[string]string{vnic: "my-nlb"}}

	EnrichRecords(context.Background(), records, resolver)

	assert.Equal(t, 1, resolver.callCount, "ResolveMany should be called exactly once per invocation")
	assert.Equal(t, []string{vnic}, resolver.ocidsReceived, "three records with the same OCID should dedupe to one entry")
}

func TestEnrichRecords_SkipsResolveWhenNothingNeedsIt(t *testing.T) {
	records := common.OCILoggingEvent{bastionGetBastionRecord(), bastionGetBastionRecord()}
	resolver := &fakeResolver{}

	EnrichRecords(context.Background(), records, resolver)

	assert.Zero(t, resolver.callCount, "resolver should never be called when every record already has a name")
	for _, rec := range records {
		assert.Equal(t, "demo-bastion", dataOf(t, rec)["logging.oci.displayName"])
	}
}

func TestEnrichRecords_InjectsResolvedName(t *testing.T) {
	const vnic = "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq"
	rec := vnicRecord(vnic)
	records := common.OCILoggingEvent{rec}
	resolver := &fakeResolver{resolved: map[string]string{vnic: "my-nlb"}}

	EnrichRecords(context.Background(), records, resolver)

	assert.Equal(t, "my-nlb", dataOf(t, rec)["logging.oci.displayName"])
}

func TestEnrichRecords_PartialResolveFailureStillInjectsWhatResolved(t *testing.T) {
	firewallRec := map[string]interface{}{
		"logContent": map[string]interface{}{
			"type": "com.oraclecloud.networkfirewall.traffic",
			"data": map[string]interface{}{"firewall-id": "ocid1.networkfirewall.oc1.iad.a"},
		},
	}
	dnsRec := map[string]interface{}{
		"logContent": map[string]interface{}{
			"type":   "com.oraclecloud.dns.private.resolver",
			"source": "ocid1.dnsresolver.oc1.iad.b",
		},
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

func TestEnrichRecords_NilResolverStillInjectsAlreadyKnownNames(t *testing.T) {
	const vnic = "ocid1.vnic.oc1.iad.c"
	bastionRec := bastionGetBastionRecord()
	vnicRec := vnicRecord(vnic)
	records := common.OCILoggingEvent{bastionRec, vnicRec}

	assert.NotPanics(t, func() {
		EnrichRecords(context.Background(), records, nil)
	})

	assert.Equal(t, "demo-bastion", dataOf(t, bastionRec)["logging.oci.displayName"], "name already known from the payload, no resolver needed")
	assert.NotContains(t, dataOf(t, vnicRec), "logging.oci.displayName", "no resolver was available, so this can't have gotten a name")
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
		"logContent": map[string]interface{}{
			"type": "com.oraclecloud.KeyManagementService.GetVault",
			// deliberately no "data" key at all, unlike unmatchedVaultRecord()
		},
	}
	records := common.OCILoggingEvent{rec}

	EnrichRecords(context.Background(), records, &fakeResolver{})

	logContent := rec["logContent"].(map[string]interface{})
	_, dataExists := logContent["data"]
	assert.False(t, dataExists, "injectResourceName's early return means a data map is never created when there's nothing to write")
}

func TestResolveTimeout(t *testing.T) {
	defer os.Unsetenv(common.ResourceResolveTimeoutSeconds)

	os.Unsetenv(common.ResourceResolveTimeoutSeconds)
	assert.Equal(t, 8, int(resolveTimeout().Seconds()), "default should be DefaultResourceResolveTimeoutSeconds")

	os.Setenv(common.ResourceResolveTimeoutSeconds, "30")
	assert.Equal(t, 30, int(resolveTimeout().Seconds()), "a valid override should be honored")

	os.Setenv(common.ResourceResolveTimeoutSeconds, "not-a-number")
	assert.Equal(t, 8, int(resolveTimeout().Seconds()), "an invalid override should fall back to the default")

	os.Setenv(common.ResourceResolveTimeoutSeconds, "-5")
	assert.Equal(t, 8, int(resolveTimeout().Seconds()), "a non-positive override should fall back to the default")
}
