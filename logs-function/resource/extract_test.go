package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtract covers every rule in rules.go against a record shaped like the real sample it was
// built from, plus the types that are deliberately NOT in rules.go (either because they always
// carry a name natively -- API Gateway, Queue, Streaming -- or because they're genuinely
// unverified/excluded -- Vault, Compute list operations, VCN flow logs, Bastion (no confirmed
// entity-definitions rule for either)).
func TestExtract(t *testing.T) {
	tests := []struct {
		name     string
		logData  map[string]interface{}
		expected Extraction
	}{
		{
			name: "NLB connection log (VCN flow logs) is unmatched -- excluded, no confirmed entity rule",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.vcn.flowlogs.DataEvent",
					"oracle": map[string]interface{}{
						"resourceId":   "ocid1.networkloadbalancer.oc1.iad.amaaaaaatvlqdbyao2zbb3jidgoxqbzid6vqr2xqlib336onma5h32oew22a",
						"resourceType": "networkloadbalancer",
						"vnicocid":     "ocid1.vnic.oc1.iad.abuwcljrhxeeyawuc5qsdv5opaosn26o5fftjdkdcgaqjn6ka4rqc3wih3bq",
					},
					"source": "-",
					"data":   map[string]interface{}{"flowid": "66548b44"},
				},
			},
			expected: Extraction{},
		},
		{
			name: "VNIC ACCEPT flow log (loadbalancer variant) is unmatched the same way",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.vcn.flowlogs.DataEvent",
					"oracle": map[string]interface{}{
						"resourceId":   "ocid1.loadbalancer.oc1.iad.aaaaaaaanptlxsdlxzfshbphyg33hhe2ypmmzaen5jlhlx2igwqhsijdqqlq",
						"resourceType": "loadbalancer",
						"vnicocid":     "ocid1.vnic.oc1.iad.abuwcljtsxsa7nu6spexecas25aw33gqufj2ljccwocl34xqcl6hhstcclta",
					},
					"source": "-",
					"data":   map[string]interface{}{"action": "ACCEPT"},
				},
			},
			expected: Extraction{},
		},
		{
			name: "Network Firewall traffic log resolves via data.firewall-id",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.networkfirewall.traffic",
					"data": map[string]interface{}{
						"firewall-id": "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq",
						"action":      "allow",
					},
					"source": "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq",
				},
			},
			expected: Extraction{
				OCID:         "ocid1.networkfirewall.oc1.iad.amaaaaaatvlqdbya2vdgj5rqg7zw52o7qq2a5z7k6lu2ubefwf7ykqwei4xq",
				NeedsResolve: true,
			},
		},
		{
			name: "DNS Private Resolver resolves via source",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.dns.private.resolver",
					"source": "ocid1.dnsresolver.oc1.iad.amaaaaaatvlqdbyaf4mgekhmbsvoxvyluwzhowzpdci4rzjrgat6n7z2yj4q",
					"data":   map[string]interface{}{"qname": "auth.us-ashburn-1.oraclecloud.com."},
				},
			},
			expected: Extraction{
				OCID:         "ocid1.dnsresolver.oc1.iad.amaaaaaatvlqdbyaf4mgekhmbsvoxvyluwzhowzpdci4rzjrgat6n7z2yj4q",
				NeedsResolve: true,
			},
		},
		{
			name: "Integration Cloud activity stream resolves via source (prefix match)",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.integration.integrationinstance.activitystream",
					"source": "ocid1.integrationinstance.oc1.iad.amaaaaaatvlqdbyassm3eanvviwuoijb57xkalojoiovyfuu6lslkmx7b6lq",
					"data":   map[string]interface{}{"actionName": "LogTestEndPoint"},
				},
			},
			expected: Extraction{
				OCID:         "ocid1.integrationinstance.oc1.iad.amaaaaaatvlqdbyassm3eanvviwuoijb57xkalojoiovyfuu6lslkmx7b6lq",
				NeedsResolve: true,
			},
		},
		{
			name: "Events Service rule-execution log resolves via data.ruleId, source is ignored (no NamePath)",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.eventsservice.eventrule.ruleexecutionlog",
					"source": "poc-reconciler-rule",
					"data": map[string]interface{}{
						"ruleId":  "ocid1.eventrule.oc1.iad.amaaaaaatvlqdbyacot7p5fphd6pbcuz3zt2x7jgc4fl7xxqlrhqmoz6gjfq",
						"message": "Rule has matched event",
					},
				},
			},
			expected: Extraction{
				OCID:         "ocid1.eventrule.oc1.iad.amaaaaaatvlqdbyacot7p5fphd6pbcuz3zt2x7jgc4fl7xxqlrhqmoz6gjfq",
				NeedsResolve: true, // no NamePath declared, even though source looks like a name
			},
		},
		{
			name: "Bastion GetBastion is unmatched -- excluded, no confirmed entity rule",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.bastion.GetBastion",
					"source": "demo-bastion",
					"data": map[string]interface{}{
						"resourceId": "ocid1.bastion.oc1.iad.amaaaaaatvlqdbyagt36dlcwb6zdma3ddbix74hdcge5xvfnewy6heaovyjq",
						"eventName":  "GetBastion",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "Bastion ListSessions is unmatched -- excluded, no confirmed entity rule",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.bastion.ListSessions",
					"source": "",
					"data": map[string]interface{}{
						"resourceId": nil, // JSON null decodes to Go nil
						"eventName":  "ListSessions",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "GoldenGate resolves via data.resourceId; source duplicates the OCID, not a name",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.goldengate.deployment.restapi_logs",
					"source": "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca",
					"data": map[string]interface{}{
						"resourceId": "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca",
					},
				},
			},
			expected: Extraction{
				OCID:         "ocid1.goldengatedeployment.oc1.iad.amaaaaaaev3nvkqaafhnn35jb6xoomvlfhonb2gqzqb5tpf2h45b62p3gyca",
				NeedsResolve: true,
			},
		},
		{
			name: "API Gateway is unmatched -- always has data.gatewayDisplayName natively, deliberately no rule",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.apigateway.apideployment.access",
					"data": map[string]interface{}{
						"gatewayId":          "ocid1.apigateway.oc1.iad.amaaaaaatvlqdbyav66p2mzayoyiz33nrs7wm5ya6bjxwgiso7qnmupsszcq",
						"gatewayDisplayName": "jashraf-gateway",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "Vault GetVault is unmatched -- excluded entirely",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.KeyManagementService.GetVault",
					"data": map[string]interface{}{
						"resourceId": "ocid1.vault.oc1.iad.ejurnwh5aabpq.abuwcljsa2gl3wxoj2ftbtwijb64qumg5dj3zbor6oasactkqqcngcjoszra",
						"eventName":  "GetVault",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "Vault ListSecrets is unmatched with a null resourceId -- still just unmatched, no special casing",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.VaultSecret.ListSecrets",
					"data": map[string]interface{}{
						"resourceId": nil,
						"eventName":  "ListSecrets",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "Compute ListInstancePoolsV2 is unmatched, resourceId null (list operation, no single resource)",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.ComputeManagement.ListInstancePoolsV2",
					"data": map[string]interface{}{
						"resourceId": nil,
						"eventName":  "ListInstancePoolsV2",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "Queue CreateQueue is unmatched -- always has a name via source, dropped rule",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.queueapi.CreateQueue.begin",
					"source": "spike-test-queue",
					"data": map[string]interface{}{
						"resourceId": "ocid1.queue.oc1.iad.amaaaaaatvlqdbya5hef3vtahnl232tjefo6erknl4qhz7wzunoh7m3idfda",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "Streaming createStream is unmatched -- always has a name via source, dropped rule",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type":   "com.oraclecloud.Streaming-ControlPlane.createStream",
					"source": "spike-test-stream",
					"data": map[string]interface{}{
						"resourceId": "ocid1.stream.oc1.iad.amaaaaaatvlqdbya3uv52dfl343y3fgtcbk7mcfqhn3g2qxezrbunov3rqlq",
					},
				},
			},
			expected: Extraction{},
		},
		{
			name: "a completely unknown future log type is unmatched",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.somefutureservice.someaction",
					"data": map[string]interface{}{"resourceId": "ocid1.somefutureresource.oc1.iad.abc"},
				},
			},
			expected: Extraction{},
		},
		{
			name:     "record missing logContent entirely",
			logData:  map[string]interface{}{"foo": "bar"},
			expected: Extraction{},
		},
		{
			name: "logContent present but type key missing",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"data": map[string]interface{}{"resourceId": "ocid1.bastion.oc1.iad.x"},
				},
			},
			expected: Extraction{},
		},
		{
			name: "matched rule but the OCID field holds a non-OCID value -- rejected, not extracted",
			logData: map[string]interface{}{
				"logContent": map[string]interface{}{
					"type": "com.oraclecloud.goldengate.deployment.restapi_logs",
					"data": map[string]interface{}{
						"resourceId": "not-an-ocid",
					},
					"source": "not-an-ocid",
				},
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
