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
	"strings"
	"time"

	fdk "github.com/fnproject/fdk-go"
)

// OCILogEvent represents the structure of log events from OCI Service Connector Hub
type OCILogEvent struct {
	Data []struct {
		ID     string    `json:"id"`
		Source string    `json:"source"`
		Type   string    `json:"type"`
		Time   time.Time `json:"time"`
		Data   struct {
			LogContent struct {
				Data   map[string]interface{} `json:"data"`
				ID     string                 `json:"id"`
				Source string                 `json:"source"`
				Type   string                 `json:"type"`
				Time   time.Time              `json:"time"`
			} `json:"logContent"`
			LogGroupID string `json:"logGroupId"`
			LogID      string `json:"logId"`
		} `json:"data"`
	} `json:"data"`
}

// NewRelicLogEntry represents a single log entry for New Relic
type NewRelicLogEntry struct {
	Timestamp  int64                  `json:"timestamp"`
	Message    string                 `json:"message"`
	Attributes map[string]interface{} `json:"attributes"`
}

// NewRelicLogPayload represents the payload structure for New Relic Logs API
type NewRelicLogPayload struct {
	Logs []NewRelicLogEntry `json:"logs"`
}

// Global variables
var (
	newRelicEndpoint  string
	newRelicIngestKey string
	httpClient        *http.Client
)

// init initializes the HTTP client and environment variables
func init() {
	// Initialize HTTP client with timeout
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get New Relic endpoint from environment
	newRelicEndpoint = os.Getenv("NEWRELIC_LOGS_ENDPOINT")
	if newRelicEndpoint == "" {
		newRelicEndpoint = "https://log-api.newrelic.com/log/v1"
	}

	// Get New Relic ingest key from environment
	newRelicIngestKey = os.Getenv("NEWRELIC_INGEST_KEY")
	if newRelicIngestKey == "" {
		log.Fatal("NEWRELIC_INGEST_KEY environment variable is required")
	}

	log.Printf("Function initialized successfully with endpoint: %s", newRelicEndpoint)
}

// sanitizeAttributeKey sanitizes keys for New Relic attributes.
func sanitizeAttributeKey(key string) string {
	// Replace invalid characters with underscores.
	sanitized := strings.ReplaceAll(key, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, " ", "_")
	return strings.ToLower(sanitized)
}

// transformOCILogsToNewRelic converts OCI log events to New Relic format
func transformOCILogsToNewRelic(ociEvent OCILogEvent) []NewRelicLogEntry {
	var newRelicLogs []NewRelicLogEntry

	for _, event := range ociEvent.Data {
		logContent := event.Data.LogContent

		// Extract message from log data
		message := ""
		if msg, ok := logContent.Data["message"]; ok {
			if msgStr, ok := msg.(string); ok {
				message = msgStr
			}
		}

		// If no message field, use the entire data as JSON string
		if message == "" {
			if dataBytes, err := json.Marshal(logContent.Data); err == nil {
				message = string(dataBytes)
			}
		}

		// Create attributes map
		attributes := make(map[string]interface{})

		// Add OCI-specific attributes
		attributes["oci.log.id"] = event.Data.LogID
		attributes["oci.log.group.id"] = event.Data.LogGroupID
		attributes["oci.log.source"] = logContent.Source
		attributes["oci.log.type"] = logContent.Type
		attributes["oci.event.id"] = event.ID
		attributes["oci.event.source"] = event.Source
		attributes["oci.event.type"] = event.Type

		// Add all log data as attributes (flatten if possible)
		for key, value := range logContent.Data {
			if key != "message" { // Avoid duplicating message
				attributes[fmt.Sprintf("log.%s", sanitizeAttributeKey(key))] = value
			}
		}

		// Create New Relic log entry
		newRelicLog := NewRelicLogEntry{
			Timestamp:  logContent.Time.UnixMilli(),
			Message:    message,
			Attributes: attributes,
		}

		newRelicLogs = append(newRelicLogs, newRelicLog)
	}

	return newRelicLogs
}

// sendLogsToNewRelic sends the transformed logs to New Relic
func sendLogsToNewRelic(ctx context.Context, logs []NewRelicLogEntry) error {
	if len(logs) == 0 {
		return nil
	}

	payload := NewRelicLogPayload{
		Logs: logs,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal logs to JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", newRelicEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Key", newRelicIngestKey)
	req.Header.Set("User-Agent", "OCI-LogForwarder/1.0")

	log.Printf("Sending %d logs to New Relic endpoint: %s", len(logs), newRelicEndpoint)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to New Relic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("new Relic API returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully sent %d logs to New Relic", len(logs))
	return nil
}

// HandleRequest is the main function handler for OCI Functions
func HandleRequest(ctx context.Context, in io.Reader, out io.Writer) {
	input, err := io.ReadAll(in)
	if err != nil {
		log.Printf("Failed to read input: %v", err)
		fmt.Fprintf(out, `{"error": "Failed to read input: %s"}`, err.Error())
		return
	}

	log.Printf("Received payload of size: %d bytes", len(input))

	// Parse OCI log event
	var ociEvent OCILogEvent
	if err := json.Unmarshal(input, &ociEvent); err != nil {
		log.Printf("Failed to parse OCI log event: %v", err)
		fmt.Fprintf(out, `{"error": "Failed to parse input: %s"}`, err.Error())
		return
	}

	log.Printf("Parsed %d log events from OCI", len(ociEvent.Data))

	if len(ociEvent.Data) == 0 {
		log.Println("No log events to process")
		fmt.Fprint(out, `{"message": "No log events to process"}`)
		return
	}

	// Transform logs to New Relic format
	newRelicLogs := transformOCILogsToNewRelic(ociEvent)
	log.Printf("Transformed %d logs for New Relic", len(newRelicLogs))

	// Send logs to New Relic
	if err := sendLogsToNewRelic(ctx, newRelicLogs); err != nil {
		log.Printf("Failed to send logs to New Relic: %v", err)
		fmt.Fprintf(out, `{"error": "Failed to send logs: %s"}`, err.Error())
		return
	}

	response := map[string]interface{}{
		"message":          "Logs forwarded successfully",
		"processed_events": len(ociEvent.Data),
		"forwarded_logs":   len(newRelicLogs),
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}

	json.NewEncoder(out).Encode(response)
}

// main function - entry point for OCI Functions
func main() {
	fdk.Handle(fdk.HandlerFunc(HandleRequest))
}
