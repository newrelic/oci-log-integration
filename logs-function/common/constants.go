// Package common provides common constants structs and variables.
package common

// InstrumentationProvider is a parameter necessary for Entity Synthesis at New Relic.
const InstrumentationProvider = "oci"

// InstrumentationName is a parameter necessary for Entity Synthesis at New Relic.
const InstrumentationName = "log-function"

// SecretOCID is the environment variable name for the OCI secret OCID.
const SecretOCID = "SECRET_OCID"

// VaultRegion is the environment variable name for the OCI vault region.
const VaultRegion = "VAULT_REGION"

// NumberOfWorkers defines the number of concurrent worker goroutines for processing log batches.
const NumberOfWorkers = 6

// NewRelicRegion is the name of the environment variable for the New Relic region.
const NewRelicRegion = "NEW_RELIC_REGION"

// DebugEnabled is the name of the environment variable for enabling debug mode.
const DebugEnabled = "DEBUG_ENABLED"

// MaxPayloadSize is the maximum size of a payload.
// Reference: https://docs.newrelic.com/docs/logs/log-api/introduction-log-api/#limits
const MaxPayloadSize = 1 * 1024 * 1024 // 1 mb

// Environment variable names
const EnvLicenseKey = "NEW_RELIC_LICENSE_KEY"

// Secret field names
const LicenseKey = "licenseKey"
