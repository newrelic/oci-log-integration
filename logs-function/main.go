package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fnproject/fdk-go"
)

// IncomingLoggingEvent represents a SINGLE log entry from the OCI Audit Logs payload.
// This struct is updated to accurately reflect the structure of the JSON provided by the user.
type IncomingLoggingEvent struct {
	// Data field holds the core event details, now a map to handle varied and nested structures
	Data       map[string]interface{} `json:"data"`
	DataSchema string                 `json:"dataschema"`
	ID         string                 `json:"id"`
	Oracle     struct {
		CompartmentID string `json:"compartmentid"`
		IngestedTime  string `json:"ingestedtime"`
		LogGroupID    string `json:"loggroupid"`
		TenantID      string `json:"tenantid"`
	} `json:"oracle"`
	Source      string `json:"source"`
	Specversion string `json:"specversion"`
	Time        string `json:"time"` // Original time string (e.g., "2025-07-22T08:35:57.533Z")
	Type        string `json:"type"`
}

// NewRelicLogEntry New Relic Log Entry structure - NO CHANGE to this
type NewRelicLogEntry struct {
	Timestamp  int64                  `json:"timestamp,omitempty"` // Unix epoch milliseconds
	Message    string                 `json:"message,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"` // Custom attributes
}

const (
	newRelicLogsAPIEndpointUS = "https://log-api.newrelic.com/log/v1"
	newRelicLogsAPIEndpointEU = "https://log-api.eu.newrelic.com/log/v1"
)

// getURLForRegion returns the New Relic Logs API endpoint based on the specified region.
func getURLForRegion(region string) string {
	switch strings.ToLower(region) {
	case "eu":
		return newRelicLogsAPIEndpointEU
	case "us":
		return newRelicLogsAPIEndpointUS
	default:
		// Default to US if region is not specified or recognized
		return newRelicLogsAPIEndpointUS
	}
}

// main function is the entry point for the FDK (Fn Project Development Kit).
func main() {
	fdk.Handle(fdk.HandlerFunc(handleFunction))
}

// handleFunction processes the incoming OCI Audit Log events and sends them to New Relic.
func handleFunction(ctx context.Context, in io.Reader, out io.Writer) {
	// Retrieve New Relic License Key from environment variables.
	// In a production environment, this should be securely managed (e.g., OCI Vault).
	// For demonstration, a placeholder key is used.
	newRelicLicenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	if newRelicLicenseKey == "" {
		log.Println("Error: NEW_RELIC_LICENSE_KEY environment variable not set")
		return
	}

	// Retrieve New Relic Region from environment variables.
	region := os.Getenv("NEW_RELIC_REGION")
	if region == "" {
		log.Println("Warning: NEW_RELIC_REGION environment variable not set, defaulting to US.")
		region = "us" // Default to US if not set
	}

	// Read the entire incoming payload into a buffer.
	payloadBytes, err := io.ReadAll(in)
	if err != nil {
		log.Printf("Error reading incoming payload: %v\n", err)
		return
	}

	// Print the raw incoming JSON payload for debugging purposes.
	log.Printf("Incoming JSON Payload:\n%s\n", string(payloadBytes))

	// Decode the incoming payload into a slice of IncomingLoggingEvent.
	// The payload is expected to be a JSON array.
	var incomingLogEvents []IncomingLoggingEvent
	if err := json.Unmarshal(payloadBytes, &incomingLogEvents); err != nil {
		log.Printf("Error decoding incoming log events payload: %v\n", err)
		// Attempt to decode as a single object if array decoding fails (for robustness)
		var singleIncomingLogEvent IncomingLoggingEvent
		if err := json.Unmarshal(payloadBytes, &singleIncomingLogEvent); err == nil {
			incomingLogEvents = append(incomingLogEvents, singleIncomingLogEvent)
			log.Println("Decoded as a single log event.")
		} else {
			log.Printf("Failed to decode as single object either: %v\n", err)
			// Removed fmt.Fprint(out, "Error processing logs: Invalid JSON format.")
			return
		}
	}

	var nrLogs []NewRelicLogEntry

	// Iterate over each incoming log event to transform it into New Relic format.
	for _, incomingLogEvent := range incomingLogEvents {
		// 1. Extract the main message from data.message
		message := ""
		if msg, ok := incomingLogEvent.Data["message"].(string); ok {
			message = msg
		} else {
			// Fallback if 'message' is not a string or missing in 'data'
			// Marshal the entire 'data' object to a string for the message
			if dataBytes, err := json.Marshal(incomingLogEvent.Data); err == nil {
				message = string(dataBytes)
			} else {
				message = "Could not extract message from log content data."
			}
		}

		// 2. Convert the 'Time' string to Unix milliseconds timestamp
		timestampMs := int64(0)
		if incomingLogEvent.Time != "" {
			t, err := time.Parse(time.RFC3339Nano, incomingLogEvent.Time)
			if err != nil {
				log.Printf("Warning: Could not parse time string '%s': %v. Using current time.", incomingLogEvent.Time, err)
				timestampMs = time.Now().UnixMilli()
			} else {
				timestampMs = t.UnixMilli()
			}
		} else {
			log.Println("Warning: 'time' field is empty. Using current time for timestamp.")
			timestampMs = time.Now().UnixMilli()
		}

		// 3. Prepare attributes for New Relic
		attributes := make(map[string]interface{})

		// Add top-level fields directly as attributes
		attributes["dataschema"] = incomingLogEvent.DataSchema
		attributes["id"] = incomingLogEvent.ID
		attributes["source"] = incomingLogEvent.Source
		attributes["specversion"] = incomingLogEvent.Specversion
		attributes["type"] = incomingLogEvent.Type

		// Add Oracle-specific metadata with "oci_" prefix
		attributes["oci_compartment_id"] = incomingLogEvent.Oracle.CompartmentID
		attributes["oci_ingested_time"] = incomingLogEvent.Oracle.IngestedTime
		attributes["oci_log_group_id"] = incomingLogEvent.Oracle.LogGroupID
		attributes["oci_tenant_id"] = incomingLogEvent.Oracle.TenantID
		// Note: The original 'logid' and 'regionId' from the first struct were not in the new JSON example.
		// If they are needed, they would need to be added to IncomingLoggingEvent and then here.

		// Flatten the 'data' map into attributes, using dot notation for nested fields.
		// Exclude the 'message' field as it's already the main log message.
		flattenMap(incomingLogEvent.Data, "", attributes, "message")

		nrLogs = append(nrLogs, NewRelicLogEntry{
			Timestamp:  timestampMs,
			Message:    message,
			Attributes: attributes,
		})
	}

	if len(nrLogs) == 0 {
		log.Println("No logs to send to New Relic (after parsing and transformation).")
		// Removed fmt.Fprint(out, "No logs to process.")
		return
	}

	// Marshal the transformed New Relic log entries into JSON.
	newRelicPayloadBytes, err := json.Marshal(nrLogs)
	if err != nil {
		log.Printf("Error marshaling New Relic log payload: %v\n", err)
		// Removed fmt.Fprint(out, "Error marshaling New Relic payload.")
		return
	}

	// For debugging, print the final New Relic payload.
	log.Printf("New Relic Payload:\n%s\n", string(newRelicPayloadBytes))

	// Send logs to New Relic via HTTP POST request.
	req, err := http.NewRequest("POST", getURLForRegion(region), bytes.NewBuffer(newRelicPayloadBytes))
	if err != nil {
		log.Printf("Error creating HTTP request to New Relic: %v\n", err)
		// Removed fmt.Fprint(out, "Error creating HTTP request.")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Key", newRelicLicenseKey) // New Relic uses X-License-Key or Api-Key

	client := &http.Client{Timeout: 30 * time.Second} // Set a reasonable timeout for the HTTP client
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending logs to New Relic: %v\n", err)
		// Removed fmt.Fprint(out, "Error sending logs to New Relic.")
		return
	}
	defer resp.Body.Close() // Ensure the response body is closed

	// Check the HTTP response status code. New Relic typically returns 200 OK or 202 Accepted.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		responseBody, _ := io.ReadAll(resp.Body)
		log.Printf("New Relic API returned non-2xx status: %d, Response: %s\n", resp.StatusCode, string(responseBody))
		// Removed fmt.Fprintf(out, "New Relic API error: Status %d, Response: %s", resp.StatusCode, string(responseBody))
		return
	}

	log.Printf("Successfully sent %d logs to New Relic. Status: %d", len(nrLogs), resp.StatusCode)
	// Removed fmt.Fprint(out, "Logs successfully sent to New Relic.")
}

// flattenMap recursively flattens a nested map into a single-level map with dot-separated keys.
// It takes the source map, a prefix for the current level, the result map, and keys to exclude.
func flattenMap(source map[string]interface{}, prefix string, result map[string]interface{}, excludeKeys ...string) {
	for k, v := range source {
		// Check if the current key should be excluded
		isExcluded := false
		for _, exKey := range excludeKeys {
			if k == exKey {
				isExcluded = true
				break
			}
		}
		if isExcluded {
			continue // Skip this key
		}

		newKey := k
		if prefix != "" {
			newKey = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			// Recursively flatten nested maps
			flattenMap(val, newKey, result)
		case []interface{}:
			// Handle slices: if elements are simple, keep as slice; if complex, stringify or handle individually
			// For simplicity, for now, we'll add them as is. New Relic can handle arrays of primitives.
			// If array elements are complex objects, they might need further flattening or stringification.
			result[newKey] = val
		case float64:
			// JSON numbers are typically decoded as float64 in Go.
			// Check if it's an integer for cleaner representation if applicable.
			if val == float64(int(val)) {
				result[newKey] = int(val)
			} else {
				result[newKey] = val
			}
		case json.Number:
			// If using json.Decoder.UseNumber(), handle json.Number
			if i, err := val.Int64(); err == nil {
				result[newKey] = i
			} else if f, err := val.Float64(); err == nil {
				result[newKey] = f
			} else {
				result[newKey] = val.String() // Fallback to string
			}
		default:
			result[newKey] = val
		}
	}
}

// Helper to convert string to int64, useful for timestamps if needed
func stringToInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0 // Or handle error appropriately
	}
	return i
}
