package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"encoding/base64"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/fnproject/fdk-go"
	"github.com/newrelic/newrelic-client-go/v2/pkg/region"
	"github.com/newrelic/newrelic-client-go/v2/pkg/config"
	logging "github.com/newrelic/newrelic-client-go/v2/pkg/logs"
)


type IncomingLoggingEvent struct {
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
	Time        string `json:"time"` 
	Type        string `json:"type"`
}

type NewRelicLogEntry struct {
	Timestamp  int64                  `json:"timestamp,omitempty"`
	Message    string                 `json:"message,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"` 
}

const (
	newRelicLogsAPIEndpointUS = "https://log-api.newrelic.com/log/v1"
	newRelicLogsAPIEndpointEU = "https://log-api.eu.newrelic.com/log/v1"
)

func getURLForRegion(region string) string {
	switch strings.ToLower(region) {
	case "eu":
		return newRelicLogsAPIEndpointEU
	case "us":
		return newRelicLogsAPIEndpointUS
	default:
		return newRelicLogsAPIEndpointUS
	}
}

func main() {
	fdk.Handle(fdk.HandlerFunc(handleFunction))
}

func handleFunction(ctx context.Context, in io.Reader, out io.Writer) {
	nrClient, err := NewNRClient()

	payloadBytes, err := io.ReadAll(in)
	if err != nil {
		log.Printf("Error reading incoming payload: %v\n", err)
		return
	}

	// log.Printf("Incoming JSON Payload:\n%s\n", string(payloadBytes))

	var incomingLogEvents []IncomingLoggingEvent
	if err := json.Unmarshal(payloadBytes, &incomingLogEvents); err != nil {
		log.Printf("Error decoding incoming log events payload: %v\n", err)
		var singleIncomingLogEvent IncomingLoggingEvent
		if err := json.Unmarshal(payloadBytes, &singleIncomingLogEvent); err == nil {
			incomingLogEvents = append(incomingLogEvents, singleIncomingLogEvent)
			log.Println("Decoded as a single log event.")
		} else {
			log.Printf("Failed to decode as single object either: %v\n", err)
			return
		}
	}

	var nrLogs []NewRelicLogEntry

	for _, incomingLogEvent := range incomingLogEvents {
		message := ""
		if msg, ok := incomingLogEvent.Data["message"].(string); ok {
			message = msg
		} else {
			if dataBytes, err := json.Marshal(incomingLogEvent.Data); err == nil {
				message = string(dataBytes)
			} else {
				message = "Could not extract message from log content data."
			}
		}

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

		attributes := make(map[string]interface{})

		attributes["dataschema"] = incomingLogEvent.DataSchema
		attributes["id"] = incomingLogEvent.ID
		attributes["source"] = incomingLogEvent.Source
		attributes["specversion"] = incomingLogEvent.Specversion
		attributes["type"] = incomingLogEvent.Type

		attributes["oci_compartment_id"] = incomingLogEvent.Oracle.CompartmentID
		attributes["oci_ingested_time"] = incomingLogEvent.Oracle.IngestedTime
		attributes["oci_log_group_id"] = incomingLogEvent.Oracle.LogGroupID
		attributes["oci_tenant_id"] = incomingLogEvent.Oracle.TenantID

		flattenMap(incomingLogEvent.Data, "", attributes, "message")

		nrLogs = append(nrLogs, NewRelicLogEntry{
			Timestamp:  timestampMs,
			Message:    message,
			Attributes: attributes,
		})
	}

	if len(nrLogs) == 0 {
		log.Println("No logs to send to New Relic (after parsing and transformation).")
		return
	}

	err = nrClient.CreateLogEntry(nrLogs)

	if err != nil {
		log.Printf("Error sending logs to New Relic: %v\n", err)
	} else {
		log.Printf("Successfully sent %d logs to New Relic.", len(nrLogs))
	}
}

func flattenMap(source map[string]interface{}, prefix string, result map[string]interface{}, excludeKeys ...string) {
	for k, v := range source {
		isExcluded := false
		for _, exKey := range excludeKeys {
			if k == exKey {
				isExcluded = true
				break
			}
		}
		if isExcluded {
			continue 
		}

		newKey := k
		if prefix != "" {
			newKey = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			flattenMap(val, newKey, result)
		case []interface{}:
			result[newKey] = val
		case float64:
			if val == float64(int(val)) {
				result[newKey] = int(val)
			} else {
				result[newKey] = val
			}
		case json.Number:
			if i, err := val.Int64(); err == nil {
				result[newKey] = i
			} else if f, err := val.Float64(); err == nil {
				result[newKey] = f
			} else {
				result[newKey] = val.String() 
			}
		default:
			result[newKey] = val
		}
	}
}

func stringToInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

type NewRelicClientAPI interface {
	CreateLogEntry(logEntry interface{}) error
}

func NewNRClient() (NewRelicClientAPI, error) {
	nrRegion, _ := region.Get(region.Name(os.Getenv("NEW_RELIC_REGION")))
	var nrClient logging.Logs
	cfg := config.Config{
		Compression: config.Compression.Gzip,
	}

	if os.Getenv("DEBUG_ENABLED") == "true" {
		cfg.LogLevel = "debug"
	} else {
		cfg.LogLevel = "info"
	}

	if err := cfg.SetRegion(nrRegion); err != nil {
		return &nrClient, err
	}

	licenseKey, err := fetchAPIKeyFromVault(os.Getenv("SECRET_OCID"), os.Getenv("VAULT_REGION"))
	cfg.LicenseKey = licenseKey
	nrClient = logging.New(cfg)
	return &nrClient, err
}


func fetchAPIKeyFromVault(secretOCID, vaultRegion string) (string, error) {
	ctx := context.Background()

	var provider common.ConfigurationProvider
	var err error

	provider, err = auth.ResourcePrincipalConfigurationProvider()
	secretsClient, err := secrets.NewSecretsClientWithConfigurationProvider(provider)
	if err != nil {
		log.Printf("ERROR: Failed to create SecretsClient: %v", err)
		return "", fmt.Errorf("failed to create SecretsClient: %w", err)
	}

	secretsClient.SetRegion(vaultRegion)

	getSecretBundleRequest := secrets.GetSecretBundleRequest{
		SecretId: common.String(secretOCID),
	}

	scResponse, err := secretsClient.GetSecretBundle(ctx, getSecretBundleRequest)
	if err != nil {
		log.Printf("ERROR: Failed to fetch secret bundle for OCID %s: %v", secretOCID, err)
		return "", fmt.Errorf("failed to fetch secret bundle: %w", err)
	}

	secretContent, ok := scResponse.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
	if !ok {
		log.Printf("ERROR: Unexpected secret content type for OCID %s", secretOCID)
		return "", fmt.Errorf("unexpected secret content type")
	}

	if secretContent.Content == nil {
		log.Printf("ERROR: Secret content is nil for OCID %s", secretOCID)
		return "", fmt.Errorf("secret content is nil")
	}

	decodedSecret, err := base64.StdEncoding.DecodeString(*secretContent.Content)
	if err != nil {
		log.Printf("ERROR: Failed to base64 decode secret content for OCID %s: %v", secretOCID, err)
		return "", fmt.Errorf("failed to decode secret content: %w", err)
	}

	return string(decodedSecret), nil
}