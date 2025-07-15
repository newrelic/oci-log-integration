package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fnproject/fdk-go"
)

type OCICloudEvent struct {
	Specversion string `json:"specversion"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Subject     string `json:"subject"`
	ID          string `json:"id"`
	Time        string `json:"time"`
	Comptype    string `json:"comptype"`
	// The Data field contains the actual log entries from OCI Logging
	Data struct {
		CompartmentID string `json:"compartmentId"`
		LogContent    struct {
			Data []map[string]interface{} `json:"data"` // This will contain the actual log lines/objects
		} `json:"logContent"`
		// ... other fields as per your OCI log type
	} `json:"data"`
}

// NewRelicLogEntry New Relic Log Entry structure
type NewRelicLogEntry struct {
	Timestamp  int64                  `json:"timestamp"` // Unix epoch milliseconds
	Message    string                 `json:"message"`
	Attributes map[string]interface{} `json:"attributes,omitempty"` // Custom attributes
}

const (
	newRelicLogsAPIEndpointUS = "https://log-api.newrelic.com/log/v1"
	newRelicLogsAPIEndpointEU = "https://log-api.eu.newrelic.com/log/v1"
)

func main() {
	fdk.Handle(fdk.HandlerFunc(handleFunction))
}

func handleFunction(ctx context.Context, in io.Reader, out io.Writer) {
	newRelicLicenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	if newRelicLicenseKey == "" {
		log.Println("Error: NEW_RELIC_LICENSE_KEY environment variable not set.")
		return
	}

	var ociEvent OCICloudEvent
	if err := json.NewDecoder(in).Decode(&ociEvent); err != nil {
		log.Printf("Error decoding OCI event payload: %v\n", err)
		return
	}

	var nrLogs []NewRelicLogEntry

	// Process each log entry received from OCI Logging
	for _, logData := range ociEvent.Data.LogContent.Data {
		// Attempt to extract the log message. OCI logs can vary in structure.
		// You'll need to adapt this parsing logic based on the specific OCI log types you're ingesting.
		var message string
		if msg, ok := logData["data"].(map[string]interface{})["message"].(string); ok {
			message = msg
		} else if fullLog, ok := logData["data"].(string); ok {
			message = fullLog // For simpler string-based logs
		} else {
			if byteMsg, err := json.Marshal(logData); err == nil {
				message = string(byteMsg)
			} else {
				message = fmt.Sprintf("Could not extract message from log: %v", logData)
			}
		}

		timestampMs := time.Now().UnixNano() / int64(time.Millisecond) // Default to current time in milliseconds
		if t, ok := logData["time"].(string); ok {
			parsedTime, err := time.Parse(time.RFC3339Nano, t) // Adjust a format if needed
			if err == nil {
				timestampMs = parsedTime.UnixNano() / int64(time.Millisecond)
			} else {
				log.Printf("Warning: Could not parse log timestamp '%s', using current time. Error: %v\n", t, err)
			}
		} else if tNum, ok := logData["time"].(float64); ok { // Sometimes time might be a float/number
			timestampMs = int64(tNum)
		}

		attributes := make(map[string]interface{})
		attributes["oci_source_type"] = ociEvent.Source
		attributes["oci_subject"] = ociEvent.Subject
		attributes["oci_compartment_id"] = ociEvent.Data.CompartmentID
		for k, v := range logData {
			if k != "time" && k != "data" {
				attributes[fmt.Sprintf("oci_%s", k)] = v
			}
		}

		nrLogs = append(nrLogs, NewRelicLogEntry{
			Timestamp:  timestampMs,
			Message:    message,
			Attributes: attributes,
		})
	}

	if len(nrLogs) == 0 {
		log.Println("No logs to send to New Relic.")
		fmt.Fprint(out, "No logs to process.")
		return
	}

	payloadBytes, err := json.Marshal(nrLogs)
	if err != nil {
		log.Printf("Error marshaling New Relic log payload: %v\n", err)
		return
	}

	// Send logs to New Relic
	req, err := http.NewRequest("POST", newRelicLogsAPIEndpointUS, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("Error creating HTTP request to New Relic: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Key", newRelicLicenseKey) // or "Api-Key"

	client := &http.Client{Timeout: 10 * time.Second} // Set a timeout
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending logs to New Relic: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		responseBody, _ := io.ReadAll(resp.Body)
		log.Printf("New Relic API returned non-2xx status: %d, Response: %s\n", resp.StatusCode, string(responseBody))
		return
	}

	log.Printf("Successfully sent %d logs to New Relic. Status: %d\n", len(nrLogs), resp.StatusCode)
	fmt.Fprint(out, "Logs successfully sent to New Relic.")
}
