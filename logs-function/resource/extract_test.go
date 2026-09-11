package resource

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtract covers matchRule/getOCID logic for cases no real sample below exercises: types
// deliberately NOT in rules.go (either because they always carry a name natively -- API Gateway,
// Queue, Streaming -- or because they're genuinely unverified/excluded -- Vault, Compute list
// operations), plus structural edge cases (missing type field, a non-OCID value in an OCID
// field). Every rule that DOES match something is instead verified against a real captured
// payload in TestExtract_RealSamples_Clean/Reconstructed below -- testing the same rule twice,
// once synthetically and once for real, would just be duplicate coverage. Records are the
// CloudEvent directly, matching what Service Connector Hub actually delivers -- no
// logContent/datetime wrapper.
func TestExtract(t *testing.T) {
	tests := []struct {
		name     string
		logData  map[string]interface{}
		expected Extraction
	}{
		{
			name: "API Gateway is unmatched -- always has data.gatewayDisplayName natively, deliberately no rule",
			logData: map[string]interface{}{
				"type": "com.oraclecloud.apigateway.apideployment.access",
				"data": map[string]interface{}{
					"gatewayId":          "ocid1.apigateway.oc1.iad.amaaaaaatvlqdbyav66p2mzayoyiz33nrs7wm5ya6bjxwgiso7qnmupsszcq",
					"gatewayDisplayName": "jashraf-gateway",
				},
			},
			expected: Extraction{},
		},
		{
			name: "Vault GetVault is unmatched -- excluded entirely",
			logData: map[string]interface{}{
				"type": "com.oraclecloud.KeyManagementService.GetVault",
				"data": map[string]interface{}{
					"resourceId": "ocid1.vault.oc1.iad.ejurnwh5aabpq.abuwcljsa2gl3wxoj2ftbtwijb64qumg5dj3zbor6oasactkqqcngcjoszra",
					"eventName":  "GetVault",
				},
			},
			expected: Extraction{},
		},
		{
			name: "Vault ListSecrets is unmatched with a null resourceId -- still just unmatched, no special casing",
			logData: map[string]interface{}{
				"type": "com.oraclecloud.VaultSecret.ListSecrets",
				"data": map[string]interface{}{
					"resourceId": nil,
					"eventName":  "ListSecrets",
				},
			},
			expected: Extraction{},
		},
		{
			name: "Compute ListInstancePoolsV2 is unmatched, resourceId null (list operation, no single resource)",
			logData: map[string]interface{}{
				"type": "com.oraclecloud.ComputeManagement.ListInstancePoolsV2",
				"data": map[string]interface{}{
					"resourceId": nil,
					"eventName":  "ListInstancePoolsV2",
				},
			},
			expected: Extraction{},
		},
		{
			name: "Queue CreateQueue is unmatched -- always has a name via source, dropped rule",
			logData: map[string]interface{}{
				"type":   "com.oraclecloud.queueapi.CreateQueue.begin",
				"source": "spike-test-queue",
				"data": map[string]interface{}{
					"resourceId": "ocid1.queue.oc1.iad.amaaaaaatvlqdbya5hef3vtahnl232tjefo6erknl4qhz7wzunoh7m3idfda",
				},
			},
			expected: Extraction{},
		},
		{
			name: "Streaming createStream is unmatched -- always has a name via source, dropped rule",
			logData: map[string]interface{}{
				"type":   "com.oraclecloud.Streaming-ControlPlane.createStream",
				"source": "spike-test-stream",
				"data": map[string]interface{}{
					"resourceId": "ocid1.stream.oc1.iad.amaaaaaatvlqdbya3uv52dfl343y3fgtcbk7mcfqhn3g2qxezrbunov3rqlq",
				},
			},
			expected: Extraction{},
		},
		{
			name: "a completely unknown future log type is unmatched",
			logData: map[string]interface{}{
				"type": "com.oraclecloud.somefutureservice.someaction",
				"data": map[string]interface{}{"resourceId": "ocid1.somefutureresource.oc1.iad.abc"},
			},
			expected: Extraction{},
		},
		{
			name:     "record missing a type field entirely",
			logData:  map[string]interface{}{"foo": "bar"},
			expected: Extraction{},
		},
		{
			name: "matched rule but the OCID field holds a non-OCID value -- rejected, not extracted",
			logData: map[string]interface{}{
				"type": "com.oraclecloud.goldengate.deployment.restapi_logs",
				"data": map[string]interface{}{
					"resourceId": "not-an-ocid",
				},
				"source": "not-an-ocid",
			},
			// OCID fails the ocid1. prefix check, so ends up empty; no NamePath is declared for
			// this type, so there's nothing else to fall back on either.
			expected: Extraction{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Extract(tt.logData))
		})
	}
}

// Real-world sample payloads used to test Extract() against genuine records, not just the
// synthetic table above. Two groups below, by provenance: samples that were shared as clean,
// valid JSON as-is, and samples that were originally pasted in a non-JSON "key / newline / value"
// rendering (a copy/paste artifact from whatever viewer they were captured through) and had to be
// reconstructed here -- every field this package actually reads is preserved verbatim from the
// original; irrelevant nested blocks (identity/request/response/etc.) are collapsed to {} since
// nothing under test reads into them. These same samples get run through EnrichRecords in
// enrich_test.go, both one record at a time and combined into one mixed multi-record batch (the
// shape a real invocation actually takes).
//
// Each sample is the CloudEvent directly (type/data/source/oracle at the top level), matching
// what Service Connector Hub actually delivers to this function -- confirmed this session via a
// captured real record. The "datetime"/"logContent"/"regionId" wrapper seen in OCI Console's own
// log-viewer sample exports is a presentation artifact of that viewer, not the real wire shape.

const sampleNLBConnectionLog = `{
  "id": "66548b44",
  "time": "2026-08-07T11:34:30Z",
  "oracle": {
    "compartmentid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "ingestedtime": "2026-08-07T11:35:40.501Z",
    "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaatvlqdbyaoehcfmw3qgy7zhx6bhbpsd5ptj64gjdooxv3lzankgna",
    "logid": "ocid1.log.oc1.iad.amaaaaaatvlqdbyat52rqvbizdrockgdsa3uxdjthsj6cpu57z3qdwuepaea",
    "managed": "true",
    "publicIpv4": "150.136.215.207",
    "resourceId": "ocid1.networkloadbalancer.oc1.iad.amaaaaaatvlqdbyao2zbb3jidgoxqbzid6vqr2xqlib336onma5h32oew22a",
    "resourceType": "networkloadbalancer",
    "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
    "vcnOcid": "ocid1.vcn.oc1.iad.amaaaaaatvlqdbya53zycbiqmuty5ozbvl5qmnzkobizvwpzbpc2meb6x5ha",
    "vniccompartmentocid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "vnicocid": "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq",
    "vnicsubnetocid": "ocid1.subnet.oc1.iad.aaaaaaaastfwmbn2wvf3jta5b6t22m7gia27t4macjlsqm2h7mvesawdisua"
  },
  "source": "-",
  "specversion": "1.0",
  "subject": "-",
  "type": "com.oraclecloud.vcn.flowlogs.DataEvent",
  "data": {
    "flowid": "66548b44",
    "sourceAddress": "167.234.221.222",
    "destinationAddress": "10.0.0.201",
    "action": "ACCEPT"
  }
}`

const sampleNetworkFirewallTraffic = `{
  "data": {
    "action": "allow",
    "device_name": "PA-VM",
    "dport": "22",
    "dst": "10.0.0.40",
    "firewall-id": "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq",
    "proto": "tcp",
    "src": "149.118.48.73"
  },
  "id": "644237ee-9d51-4a7c-957e-3bd38dfe91ae",
  "oracle": {
    "compartmentid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "ingestedtime": "2026-08-10T18:36:14.510Z",
    "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaatvlqdbyaoehcfmw3qgy7zhx6bhbpsd5ptj64gjdooxv3lzankgna",
    "logid": "ocid1.log.oc1.iad.amaaaaaatvlqdbyahs6yjhekngau534hfsjglcwe4ra7u7cbckg4xohcr6ca",
    "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq"
  },
  "source": "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq",
  "specversion": "1.0",
  "time": "2026-08-10T18:35:50.000Z",
  "type": "com.oraclecloud.networkfirewall.traffic"
}`

const sampleVNICAccept = `{
  "id": "2aaca611",
  "time": "2026-08-07T14:23:16Z",
  "oracle": {
    "compartmentid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "ingestedtime": "2026-08-07T14:24:40.851Z",
    "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaatvlqdbyaoehcfmw3qgy7zhx6bhbpsd5ptj64gjdooxv3lzankgna",
    "logid": "ocid1.log.oc1.iad.amaaaaaatvlqdbyat52rqvbizdrockgdsa3uxdjthsj6cpu57z3qdwuepaea",
    "managed": "true",
    "publicIpv4": "143.47.100.16",
    "resourceId": "ocid1.loadbalancer.oc1.iad.aaaaaaaanptlxsdlxzfshbphyg33hhe2ypmmzaen5jlhlx2igwqhsijdqqlq",
    "resourceType": "loadbalancer",
    "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
    "vcnOcid": "ocid1.vcn.oc1.iad.amaaaaaatvlqdbya53zycbiqmuty5ozbvl5qmnzkobizvwpzbpc2meb6x5ha",
    "vniccompartmentocid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "vnicocid": "ocid1.vnic.oc1.iad.abuwcljtsxsa7nu6spexecas25aw33gqufj2ljccwocl34xqcl6hhstcclta",
    "vnicsubnetocid": "ocid1.subnet.oc1.iad.aaaaaaaastfwmbn2wvf3jta5b6t22m7gia27t4macjlsqm2h7mvesawdisua"
  },
  "source": "-",
  "specversion": "1.0",
  "subject": "-",
  "type": "com.oraclecloud.vcn.flowlogs.DataEvent",
  "data": {
    "flowid": "2aaca611",
    "sourceAddress": "140.245.43.171",
    "destinationAddress": "10.0.0.23",
    "action": "ACCEPT"
  }
}`

// sampleVNICReject deliberately shares the exact same oracle.vnicocid as sampleNLBConnectionLog
// above -- this is real, from the two original samples, and was originally used to prove dedup
// across otherwise-unrelated-looking records; both are now unmatched (no rule for VCN flow logs),
// so that dedup case no longer applies, but the samples are kept for their own "unmatched" cases.
const sampleVNICReject = `{
  "id": "8a2afced",
  "time": "2026-08-07T14:23:12Z",
  "oracle": {
    "compartmentid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "ingestedtime": "2026-08-07T14:24:36.896Z",
    "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaatvlqdbyaoehcfmw3qgy7zhx6bhbpsd5ptj64gjdooxv3lzankgna",
    "logid": "ocid1.log.oc1.iad.amaaaaaatvlqdbyat52rqvbizdrockgdsa3uxdjthsj6cpu57z3qdwuepaea",
    "managed": "true",
    "publicIpv4": "150.136.215.207",
    "resourceId": "ocid1.networkloadbalancer.oc1.iad.amaaaaaatvlqdbyao2zbb3jidgoxqbzid6vqr2xqlib336onma5h32oew22a",
    "resourceType": "networkloadbalancer",
    "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
    "vcnOcid": "ocid1.vcn.oc1.iad.amaaaaaatvlqdbya53zycbiqmuty5ozbvl5qmnzkobizvwpzbpc2meb6x5ha",
    "vniccompartmentocid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "vnicocid": "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq",
    "vnicsubnetocid": "ocid1.subnet.oc1.iad.aaaaaaaastfwmbn2wvf3jta5b6t22m7gia27t4macjlsqm2h7mvesawdisua"
  },
  "source": "-",
  "specversion": "1.0",
  "subject": "-",
  "type": "com.oraclecloud.vcn.flowlogs.DataEvent",
  "data": {
    "flowid": "8a2afced",
    "sourceAddress": "132.226.105.224",
    "destinationAddress": "10.0.0.201",
    "action": "REJECT"
  }
}`

const sampleDNSPrivateResolver = `{
  "data": {
    "answer": "[CNAME auth.us-ashburn-1.oci.oraclecloud.com.] [auth.us-ashburn-1.oci.oraclecloud.com. 5 A 140.91.13.72]",
    "qname": "auth.us-ashburn-1.oraclecloud.com.",
    "qtype": "A",
    "rcodeName": "NOERROR"
  },
  "id": "52046-1786621971-adb0ba984d1dccea",
  "oracle": {
    "compartmentid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "ingestedtime": "2026-08-13T11:53:19.668Z",
    "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaatvlqdbyacj2ok2rtwkvh62iimecx5m23yoed2mxaslxzcvqhi4na",
    "logid": "ocid1.log.oc1.iad.amaaaaaatvlqdbyav2nhfeyzd5k7ogjlsbnx3mgq7vxn3hnvvv77j5upd3dq",
    "resourceType": "dns.privateResolver",
    "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
    "vcnId": "ocid1.vcn.oc1.iad.amaaaaaatvlqdbya53zycbiqmuty5ozbvl5qmnzkobizvwpzbpc2meb6x5ha"
  },
  "source": "ocid1.dnsresolver.oc1.iad.amaaaaaatvlqdbyaf4mgekhmbsvoxvyluwzhowzpdci4rzjrgat6n7z2yj4q",
  "specversion": "1.0",
  "time": "2026-08-13T11:52:51.916Z",
  "type": "com.oraclecloud.dns.private.resolver"
}`

const sampleIntegrationActivityStream = `{
  "data": {
    "actionName": "LogTestEndPoint",
    "actionType": "Receive",
    "endPointName": "LogTestEndPoint",
    "message": "Reply to LogTestEndPoint completed"
  },
  "id": "9bdd6e7a-973f-11f1-924e-b9ae0163dbad",
  "oracle": {
    "compartmentid": "ocid1.compartment.oc1..aaaaaaaaeddxf26tf4owky2eijn6xozxepixlkci3jbnwjdttz7tbewvzb2q",
    "ingestedtime": "2026-08-13T17:51:27.908Z",
    "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaatvlqdbyagj2x4d67srtphgjn7q5fi5yzt6dinfo63adzcrjhnhta",
    "logid": "ocid1.log.oc1.iad.amaaaaaatvlqdbyak5gnpvngjodtgdvbjevezg4xf5cipnyuwsm63qiknska",
    "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq"
  },
  "source": "ocid1.integrationinstance.oc1.iad.amaaaaaatvlqdbyassm3eanvviwuoijb57xkalojoiovyfuu6lslkmx7b6lq",
  "specversion": "1.0",
  "time": "2026-08-13T17:51:23.637Z",
  "type": "com.oraclecloud.integration.integrationinstance.activitystream"
}`

// sampleGoldenGate is unwrapped relative to the original paste, which had an extra outer "data"
// key around datetime/logContent -- confirmed earlier in this project to be a paste artifact,
// not GoldenGate's real wire shape. The original's huge embedded request/response log string in
// "message" is replaced with a short placeholder; nothing under test reads that field.
const sampleGoldenGate = `{
  "data": {
    "level": "INFO",
    "message": "Request #39308: (trimmed for test brevity, not read by this package)",
    "processName": "restapi",
    "resourceId": "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca"
  },
  "id": "20260824165803.3831787590683",
  "oracle": {
    "compartmentid": "ocid1.compartment.oc1..aaaaaaaa2sss2g7zvgwe7roe3b6yicv3hfs34cr4vrv4zwhdfsb3w6ebdhxa",
    "ingestedtime": "2026-08-24T16:58:07.030Z",
    "loggroupid": "ocid1.loggroup.oc1.iad.amaaaaaaev3nvkqa6gmydxkzrhqggxhhh5snrns3s7jma7g4oa6fwng3js3q",
    "logid": "ocid1.log.oc1.iad.amaaaaaaev3nvkqaxljzghzj22y2rntsggfaqt6ksy5b2dwkubkcodhzsf4q",
    "tenantid": "ocid1.tenancy.oc1..aaaaaaaaactsxe5ptngvsrfyauc63mudq2zb4qtq2pp32t4f5jkww36kixca"
  },
  "source": "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca",
  "specversion": "1.0",
  "time": "2026-08-24T16:58:03.383Z",
  "type": "com.oraclecloud.goldengate.deployment.restapi_logs"
}`

const sampleBastionListSessions = `{
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
}`

const sampleBastionGetBastion = `{
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
}`

const sampleEventsRuleExecutionLog = `{
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
}`

// parseSample unmarshals a raw JSON sample the same way unmarshal.go does for a real incoming
// event -- into a plain map[string]interface{}, one element of common.OCILoggingEvent.
func parseSample(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &m), "sample must be valid JSON")
	return m
}

func TestExtract_RealSamples_Clean(t *testing.T) {
	const (
		firewallOCID = "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq"
		dnsOCID      = "ocid1.dnsresolver.oc1.iad.amaaaaaatvlqdbyaf4mgekhmbsvoxvyluwzhowzpdci4rzjrgat6n7z2yj4q"
		integOCID    = "ocid1.integrationinstance.oc1.iad.amaaaaaatvlqdbyassm3eanvviwuoijb57xkalojoiovyfuu6lslkmx7b6lq"
		ggOCID       = "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca"
	)

	tests := []struct {
		name     string
		raw      string
		expected Extraction
	}{
		{
			name:     "NLB Connection Log (VCN flow logs) -- unmatched, excluded, no confirmed entity rule",
			raw:      sampleNLBConnectionLog,
			expected: Extraction{},
		},
		{
			name:     "Network Firewall traffic -- resolves via data.firewall-id",
			raw:      sampleNetworkFirewallTraffic,
			expected: Extraction{OCID: firewallOCID, NeedsResolve: true},
		},
		{
			name:     "VNIC ACCEPT -- unmatched, excluded, no confirmed entity rule",
			raw:      sampleVNICAccept,
			expected: Extraction{},
		},
		{
			name:     "VNIC REJECT -- unmatched, excluded, no confirmed entity rule",
			raw:      sampleVNICReject,
			expected: Extraction{},
		},
		{
			name:     "DNS Private Resolver -- resolves via source",
			raw:      sampleDNSPrivateResolver,
			expected: Extraction{OCID: dnsOCID, NeedsResolve: true},
		},
		{
			name:     "Integration activity stream -- resolves via source",
			raw:      sampleIntegrationActivityStream,
			expected: Extraction{OCID: integOCID, NeedsResolve: true},
		},
		{
			name:     "GoldenGate -- resolves via data.resourceId, source duplicates it (not a name)",
			raw:      sampleGoldenGate,
			expected: Extraction{OCID: ggOCID, NeedsResolve: true},
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

// TestExtract_RealSamples_Reconstructed covers the samples that needed reconstruction from a
// non-JSON paste (see this file's own doc comment above).
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
