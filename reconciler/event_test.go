package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEvent(t *testing.T) {
	payload := `{
		"eventType": "com.oraclecloud.loggingservice.createloggroup",
		"source": "loggingService",
		"eventTime": "2026-07-31T10:00:00Z",
		"data": {
			"compartmentId": "ocid1.compartment.oc1..aaaa",
			"compartmentName": "team-a",
			"resourceName": "my-log-group",
			"resourceId": "ocid1.loggroup.oc1..bbbb"
		}
	}`

	evt, err := parseEvent(strings.NewReader(payload))

	require.NoError(t, err)
	assert.Equal(t, "com.oraclecloud.loggingservice.createloggroup", evt.EventType)
	assert.Equal(t, "ocid1.loggroup.oc1..bbbb", evt.Data.ResourceID)
	assert.Equal(t, "ocid1.compartment.oc1..aaaa", evt.Data.CompartmentID)
	require.NoError(t, evt.validate())
}

func TestClassify(t *testing.T) {
	cases := map[string]action{
		"com.oraclecloud.loggingservice.createloggroup": actionAdd,
		"com.oraclecloud.loggingservice.deleteloggroup": actionRemove,
		"com.oraclecloud.loggingservice.updateloggroup": actionAdd,
		"COM.ORACLECLOUD.LOGGINGSERVICE.CREATELOGGROUP": actionAdd,
		"com.oraclecloud.loggingservice.createlog":      actionUnknown,
		"":                                              actionUnknown,
	}
	for et, want := range cases {
		assert.Equalf(t, want, classify(et), "classify(%q)", et)
	}
}

func TestValidate(t *testing.T) {
	assert.Error(t, CloudEvent{EventType: "x", Data: CloudEventData{CompartmentID: "c"}}.validate(), "missing resourceId")
	assert.Error(t, CloudEvent{EventType: "x", Data: CloudEventData{ResourceID: "lg"}}.validate(), "missing compartmentId")
	assert.NoError(t, CloudEvent{EventType: "x", Data: CloudEventData{CompartmentID: "c", ResourceID: "lg"}}.validate())
}
