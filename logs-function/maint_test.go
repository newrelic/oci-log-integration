package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"
)

// Helper function to capture log output during tests
func captureLogOutput(f func()) string {
	var buf bytes.Buffer
	// Temporarily redirect log output to our buffer
	originalOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(originalOutput) // Ensure output is reset after the test

	f()
	return buf.String()
}

func TestHandleFunction_SuccessParsing(t *testing.T) {
	// Mock environment variables for New Relic (not the focus of this test, but needed by function)
	os.Setenv("NEW_RELIC_LICENSE_KEY", "dummy-key")
	os.Setenv("NEW_RELIC_REGION", "us")
	defer os.Unsetenv("NEW_RELIC_LICENSE_KEY")
	defer os.Unsetenv("NEW_RELIC_REGION")

	// The exact JSON payload, now as an ARRAY containing ONE of your sample log entries
	incomingJSON := `[
 {
   "data": {
     "additionalDetails": {
       "X-Real-Port": 14226
     },
     "availabilityDomain": "AD1",
     "compartmentId": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
     "compartmentName": "sandbox-beyond-cust-1",
     "definedTags": null,
     "eventGroupingId": "71307F2A2093465493A25661ED86E0C6/178728B3EA8B5C3F8F7C31E515FD3EC9",
     "eventName": "ListEvents",
     "freeformTags": null,
     "identity": {
       "authType": "natv",
       "callerId": null,
       "callerName": null,
       "consoleSessionId": null,
       "credentials": "ocid1.tenancy.oc1..aaaaaaaavjnm7z5ta4dgrrjw7k3mm3owqnsh4j2wgwif3y3arrgpmmdhanea/ocid1.user.oc1..aaaaaaaaa4cg35fruklu4xxbvdk7tpuzvgjzi4ki47gjthmlamrj5vyjk45q/56:71:3c:43:15:5f:0e:68:db:b4:0c:67:2d:81:72:b2",
       "ipAddress": "18.118.179.37",
       "principalId": "ocid1.user.oc1..aaaaaaaaa4cg35fruklu4xxbvdk7tpuzvgjzi4ki47gjthmlamrj5vyjk45q",
       "principalName": "Mahim Dadhich",
       "tenantId": "ocid1.tenancy.oc1..aaaaaaaavjnm7z5ta4dgrrjw7k3mm3owqnsh4j2wgwif3y3arrgpmmdhanea",
       "userAgent": "Oracle-JavaSDK/3.63.2 (Linux/5.10.228-219.884.amzn2.x86_64; Java/21.0.8; OpenJDK 64-Bit Server VM/21.0.8+9-LTS)"
     },
     "message": "ListEvents succeeded",
     "request": {
       "action": "GET",
       "headers": {
         "Accept": [
           "application/json"
         ],
         "Authorization": [
           "Signature headers=\"date (request-target) host\",keyId=\"ocid1.tenancy.oc1..aaaaaaaavjnm7z5ta4dgrrjw7k3mm3owqnsh4j2wgwif3y3arrgpmmdhanea/ocid1.user.oc1..aaaaaaaaa4cg35fruklu4xxbvdk7tpuzvgjzi4ki47gjthmlamrj5vyjk45q/56:71:3c:43:15:5f:0e:68:db:b4:0c:67:2d:81:72:b2\",algorithm=\"rsa-sha256\",signature=\"*****\",version=\"1\""
         ],
         "Connection": [
           "keep-alive"
         ],
         "Date": [
           "Tue, 22 Jul 2025 08:35:46 GMT"
         ],
         "User-Agent": [
           "Oracle-JavaSDK/3.63.2 (Linux/5.10.228-219.884.amzn2.x86_64; Java/21.0.8; OpenJDK 64-Bit Server VM/21.0.8+9-LTS)"
         ],
         "opc-client-info": [
           "Oracle-JavaSDK/3.63.2"
         ],
         "opc-client-retries": [
           "false"
         ],
         "opc-request-id": [
           "71307F2A2093465493A25661ED86E0C6"
         ]
       },
       "id": "71307F2A2093465493A25661ED86E0C6/178728B3EA8B5C3F8F7C31E515FD3EC9/0D215F7644FF9433CF3011794D9BF24E",
       "parameters": {
         "compartmentId": [
           "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq"
         ],
         "endTime": [
           "2025-07-22T08:00:49.039Z"
         ],
         "page": [
           "AP37-f_9__79-fj59fvpjP_-9fj1_fv0-Pr5-fn-__z-_PSM_fv5__3__v35-Pn1-4zMzMzM6Q=="
         ],
         "startTime": [
           "2025-07-21T13:41:17.170Z"
         ]
       },
       "path": "/20190901/auditEvents"
     },
     "resourceId": null,
     "response": {
       "headers": {
         "Content-Type": [
           "application/json"
         ],
         "Date": [
           "Tue, 22 Jul 2025 08:35:46 GMT"
         ],
         "Strict-Transport-Security": [
           "max-age=31536000; includeSubDomains;"
         ],
         "Transfer-Encoding": [
           "chunked"
         ],
         "Vary": [
           "Accept-Encoding"
         ],
         "X-Content-Type-Options": [
           "nosniff"
         ],
         "opc-next-page": [
           "AP37-f_9__7--fX89fXpjP_-9fn8-P36-vr9-vv4_Pr59fqM_fv5__3__v759fz19YzMzMzM6Q=="
         ],
         "opc-request-id": [
           "71307F2A2093465493A25661ED86E0C6/178728B3EA8B5C3F8F7C31E515FD3EC9/0D215F7644FF9433CF3011794D9BF24E"
         ]
       },
       "message": null,
       "payload": {},
       "responseTime": "2025-07-22T08:35:57.534Z",
       "status": "200"
     },
     "stateChange": {
       "current": null,
       "previous": null
     }
   },
   "dataschema": "2.0",
   "id": "93e7a143-650b-4632-8a53-a464f45ee3c5",
   "oracle": {
     "compartmentid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq",
     "ingestedtime": "2025-07-22T08:36:04.240Z",
     "loggroupid": "_Audit",
     "tenantid": "ocid1.tenancy.oc1..aaaaaaaaslaq5synueyzouxaimk3szzf66iw6od7xyiam5myn4lqhcsfu5fq"
   },
   "source": "",
   "specversion": "1.0",
   "time": "2025-07-22T08:35:57.533Z",
   "type": "com.oraclecloud.Audit.ListEvents"
 }
]`

	in := strings.NewReader(incomingJSON) // Input reader from the JSON string
	out := &bytes.Buffer{}               // Output buffer to capture function's response (will be empty now)

	// Capture log output
	logOutput := captureLogOutput(func() {
		handleFunction(context.Background(), in, out)
	})

	// --- Assertions ---

	// 1. Check if there was an error during decoding (should be nil if parsing is successful)
	if strings.Contains(logOutput, "Error decoding incoming log events payload:") {
		t.Errorf("Function reported an error during JSON decoding, but should have parsed successfully.\nLog Output:\n%s", logOutput)
	}

	// 2. Check if the raw incoming JSON was printed (from your debug code)
	if !strings.Contains(logOutput, "Incoming JSON Payload:") {
		t.Errorf("Expected 'Incoming JSON Payload:' log message, but not found.\nLog Output:\n%s", logOutput)
	}
	// Relax this assertion as the full JSON might be too big or mixed with other prints.
	// Focus on the error message absence.

	// 3. Check for the New Relic payload in the logs
	if !strings.Contains(logOutput, "New Relic Payload:") {
		t.Errorf("Expected 'New Relic Payload:' log message, but not found.\nLog Output:\n%s", logOutput)
	}
	// We expect 1 log now
	if !strings.Contains(logOutput, "Successfully sent 1 logs to New Relic. Status:") {
		t.Errorf("Expected 'Successfully sent 1 logs to New Relic' message, indicating successful processing, but got:\n%s", logOutput)
	}

	// 4. Verify that the 'out' buffer is empty, as fmt.Fprint statements were removed
	if out.Len() > 0 {
		t.Errorf("Expected 'out' buffer to be empty, but it contained: '%s'", out.String())
	}
}

// Keep TestHandleFunction_InvalidJSON and TestHandleFunction_MissingLicenseKey as is
func TestHandleFunction_InvalidJSON(t *testing.T) {
	os.Setenv("NEW_RELIC_LICENSE_KEY", "dummy-key")
	os.Setenv("NEW_RELIC_REGION", "us")
	defer os.Unsetenv("NEW_RELIC_LICENSE_KEY")
	defer os.Unsetenv("NEW_RELIC_REGION")

	invalidJSON := `{"bad_json": "missing_closing_brace"`

	in := strings.NewReader(invalidJSON)
	out := &bytes.Buffer{} // This will remain empty

	logOutput := captureLogOutput(func() {
		handleFunction(context.Background(), in, out)
	})

	// Should report a JSON decoding error in the logs
	if !strings.Contains(logOutput, "Error decoding incoming log events payload: json:") { // Updated error message
		t.Errorf("Expected JSON decoding error in logs, but got:\n%s", logOutput)
	}

	// Verify that the 'out' buffer is empty, as fmt.Fprint statements were removed
	if out.Len() > 0 {
		t.Errorf("Expected 'out' buffer to be empty, but it contained: '%s'", out.String())
	}
}

func TestHandleFunction_MissingLicenseKey(t *testing.T) {
	os.Unsetenv("NEW_RELIC_LICENSE_KEY") // Ensure it's unset for this test
	os.Setenv("NEW_RELIC_REGION", "us")
	defer os.Unsetenv("NEW_RELIC_REGION")

	// Provide a valid payload (now as an array), but expect failure due to missing key
	validJSON := `[{
        "datetime": 1753164732856,
        "logContent": {
            "data": { "message": "test" },
            "oracle": { "compartmentid": "test" }
        },
        "regionId": "us-ashburn-1"
    }]`

	in := strings.NewReader(validJSON)
	out := &bytes.Buffer{} // This will remain empty

	logOutput := captureLogOutput(func() {
		handleFunction(context.Background(), in, out)
	})

	// Should report a missing license key error in the logs
	if !strings.Contains(logOutput, "Error: NEW_RELIC_LICENSE_KEY environment variable not set.") {
		t.Errorf("Expected 'NEW_RELIC_LICENSE_KEY' error in logs, but got:\n%s", logOutput)
	}

	// Verify that the 'out' buffer is empty, as fmt.Fprint statements were removed
	if out.Len() > 0 {
		t.Errorf("Expected 'out' buffer to be empty, but it contained: '%s'", out.String())
	}
}
