package resource

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Step 1: the "clean" real-world samples -- valid JSON exactly as shared, no reconstruction
// needed. Step 2 will add the samples that were originally pasted in a non-JSON rendering and
// had to be reconstructed; step 3 will run these same samples through EnrichRecords with a fake
// resolver; step 4 will combine everything into one batch.

const sampleNLBConnectionLog = `{
  "datetime": 1786102470000,
  "logContent": {
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
  },
  "regionId": "us-ashburn-1"
}`

const sampleNetworkFirewallTraffic = `{
  "datetime": 1786386950000,
  "logContent": {
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
  },
  "regionId": "us-ashburn-1"
}`

const sampleVNICAccept = `{
  "datetime": 1786112596000,
  "logContent": {
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
  },
  "regionId": "us-ashburn-1"
}`

// sampleVNICReject deliberately shares the exact same oracle.vnicocid as sampleNLBConnectionLog
// above -- this is real, from the two original samples, and is what step 4's combined-batch test
// uses to prove dedup happens across otherwise-unrelated-looking records.
const sampleVNICReject = `{
  "datetime": 1786112592000,
  "logContent": {
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
  },
  "regionId": "us-ashburn-1"
}`

const sampleDNSPrivateResolver = `{
  "datetime": 1786621971916,
  "logContent": {
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
  }
}`

const sampleIntegrationActivityStream = `{
  "datetime": 1786643483637,
  "logContent": {
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
  }
}`

// sampleGoldenGate is unwrapped relative to the original paste, which had an extra outer "data"
// key around datetime/logContent -- confirmed earlier in this project to be a paste artifact,
// not GoldenGate's real wire shape. The original's huge embedded request/response log string in
// "message" is replaced with a short placeholder; nothing under test reads that field.
const sampleGoldenGate = `{
  "datetime": 1787590683383,
  "logContent": {
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
  }
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
		nlbVnicOCID  = "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq"
		acceptVnic   = "ocid1.vnic.oc1.iad.abuwcljtsxsa7nu6spexecas25aw33gqufj2ljccwocl34xqcl6hhstcclta"
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
			name:     "NLB Connection Log -- resolves via oracle.vnicocid",
			raw:      sampleNLBConnectionLog,
			expected: Extraction{OCID: nlbVnicOCID, NeedsResolve: true},
		},
		{
			name:     "Network Firewall traffic -- resolves via data.firewall-id",
			raw:      sampleNetworkFirewallTraffic,
			expected: Extraction{OCID: firewallOCID, NeedsResolve: true},
		},
		{
			name:     "VNIC ACCEPT -- resolves via oracle.vnicocid",
			raw:      sampleVNICAccept,
			expected: Extraction{OCID: acceptVnic, NeedsResolve: true},
		},
		{
			name:     "VNIC REJECT -- resolves via oracle.vnicocid, SAME ocid as the NLB sample",
			raw:      sampleVNICReject,
			expected: Extraction{OCID: nlbVnicOCID, NeedsResolve: true},
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
