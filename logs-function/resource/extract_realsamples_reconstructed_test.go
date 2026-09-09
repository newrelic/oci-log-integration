package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Step 2: the samples that were originally pasted in a non-JSON "key / newline / value"
// rendering (a copy/paste artifact from whatever viewer they were captured through) and had to
// be reconstructed as valid JSON here. Every field this package actually reads is preserved
// verbatim from the original; irrelevant nested blocks (identity/request/response/etc.) are
// collapsed to {} since nothing under test reads into them. Samples whose type has no rule in
// rules.go at all -- Vault, Compute, Events ListRules, Queue, and Streaming -- were removed (see
// extract_test.go's own coverage of "unmatched" behavior in general, and rules.go's own doc
// comment on why that's deliberate). The Bastion samples are kept specifically to prove Bastion
// is unmatched too, now that its rule has been removed for the same "no confirmed
// entity-definitions rule" reason.
// parseSample is defined in extract_realsamples_test.go (same package).

const sampleBastionListSessions = `{
  "datetime": 1787735384424,
  "logContent": {
    "data": {
      "additionalDetails": {},
      "availabilityDomain": "AD1",
      "compartmentId": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
      "compartmentName": "sandbox-beyond-cust-1",
      "definedTags": null,
      "eventGroupingId": "csid0f17a2ee46ce96dfbb9e16b051fb/f523a0cc69cd451792d34393864fe10a/FBD47FF78681F1BB7C0710595AB79CEE",
      "eventName": "ListSessions",
      "freeformTags": null,
      "identity": {},
      "message": "ListSessions succeeded",
      "request": {},
      "resourceId": null,
      "response": {},
      "stateChange": {}
    },
    "dataschema": "2.0",
    "id": "164a42b0-b644-4b95-9665-43c12e0b7bdb",
    "oracle": {
      "compartmentid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
      "ingestedtime": "2026-08-26T09:09:49.428Z",
      "loggroupid": "_Audit",
      "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq"
    },
    "source": "",
    "specversion": "1.0",
    "time": "2026-08-26T09:09:44.424Z",
    "type": "com.oraclecloud.bastion.ListSessions"
  }
}`

const sampleBastionGetBastion = `{
  "datetime": 1787735384403,
  "logContent": {
    "data": {
      "additionalDetails": {},
      "availabilityDomain": "AD1",
      "compartmentId": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
      "compartmentName": "sandbox-beyond-cust-1",
      "definedTags": null,
      "eventGroupingId": "csid0f17a2ee46ce96dfbb9e16b051fb/bb7c1834824c4302b6833f0939e1ec9d/4416A6F8F435D92B947B82686FC776FD",
      "eventName": "GetBastion",
      "freeformTags": null,
      "identity": {},
      "message": "demo-bastion GetBastion succeeded",
      "request": {},
      "resourceId": "ocid1.bastion.oc1.iad.amaaaaaatvlqdbyagt36dlcwb6zdma3ddbix74hdcge5xvfnewy6heaovyjq",
      "response": {},
      "stateChange": {}
    },
    "dataschema": "2.0",
    "id": "3629d469-c93b-4b12-aa12-61265250067d",
    "oracle": {
      "compartmentid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
      "ingestedtime": "2026-08-26T09:09:46.496Z",
      "loggroupid": "_Audit",
      "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq"
    },
    "source": "demo-bastion",
    "specversion": "1.0",
    "time": "2026-08-26T09:09:44.403Z",
    "type": "com.oraclecloud.bastion.GetBastion"
  }
}`

const sampleEventsRuleExecutionLog = `{
  "datetime": 1787679612000,
  "logContent": {
    "data": {
      "eventId": "c7ff46ca-b366-4278-89e7-3c70eb56baa7",
      "message": "Rule has matched event",
      "ruleId": "ocid1.eventrule.oc1.iad.amaaaaaatvlqdbyacot7p5fphd6pbcuz3zt2x7jgc4fl7xxqlrhqmoz6gjfq"
    },
    "id": "9b410252-ecfa-450d-b9ea-30cd1ca20e38",
    "oracle": {
      "compartmentid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
      "ingestedtime": "2026-08-25T17:40:19.154Z",
      "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaatvlqdbya5lw7bu2w3tvbdf7kh3de7zlnqbmlnw4t2jgle47rnchq",
      "logid": "ocid1.log.oc1.iad.amaaaaaatvlqdbyaei2b6awxj7tshbsnkjiosn4npocqgdka5xhvldcryi2q",
      "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq"
    },
    "source": "poc-reconciler-rule",
    "specversion": "1.0",
    "time": "2026-08-25T17:40:12.000Z",
    "type": "com.oraclecloud.eventsservice.eventrule.ruleexecutionlog"
  }
}`

func TestExtract_RealSamples_Reconstructed(t *testing.T) {
	const ruleOCID = "ocid1.eventrule.oc1.iad.amaaaaaatvlqdbyacot7p5fphd6pbcuz3zt2x7jgc4fl7xxqlrhqmoz6gjfq"

	tests := []struct {
		name     string
		raw      string
		expected Extraction
	}{
		{
			name:     "Bastion ListSessions -- unmatched, excluded, no confirmed entity rule",
			raw:      sampleBastionListSessions,
			expected: Extraction{},
		},
		{
			name:     "Bastion GetBastion -- unmatched, excluded, no confirmed entity rule",
			raw:      sampleBastionGetBastion,
			expected: Extraction{},
		},
		{
			name:     "Events rule-execution log -- matched, resolves via data.ruleId, source ignored (no NamePath)",
			raw:      sampleEventsRuleExecutionLog,
			expected: Extraction{OCID: ruleOCID, NeedsResolve: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := parseSample(t, tt.raw)
			got := Extract(record)
			t.Logf("extracted = %+v", got)
			t.Logf("expected  = %+v", tt.expected)
			assert.Equal(t, tt.expected, got)
		})
	}
}
