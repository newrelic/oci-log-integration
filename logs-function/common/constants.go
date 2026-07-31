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

// ClientTTL is the name of the environment variable for setting the NewRelic client cache TTL in seconds.
const ClientTTL = "CLIENT_TTL"

// DefaultClientTTL is the default TTL for the NewRelic client cache in seconds (10 minutes = 600 seconds).
const DefaultClientTTL = 600

// MaxPayloadSize is the maximum size of a payload.
// Reference: https://docs.newrelic.com/docs/logs/log-api/introduction-log-api/#limits
const MaxPayloadSize = 1 * 1024 * 1024 // 1 mb

// Secret field names
const LicenseKey = "licenseKey"

// Message channel size
const MessageChannelSize = 10

// CursorEnabled is the environment variable name that toggles per-log-group
// deduplication of records re-delivered after a Service Connector update. It is
// OFF by default; set to "true" to enable. See the cursor package.
const CursorEnabled = "CURSOR_ENABLED"

// CursorNamespace is the environment variable name for the Object Storage
// namespace that holds cursor state.
const CursorNamespace = "CURSOR_NAMESPACE"

// CursorBucket is the environment variable name for the Object Storage bucket
// that holds cursor state.
const CursorBucket = "CURSOR_BUCKET"

// CursorPrefix is the environment variable name for an optional object-name
// prefix under which cursor state is written (e.g. "nr-logs/").
const CursorPrefix = "CURSOR_PREFIX"

// CursorRegion is the environment variable name for an optional region override
// for the Object Storage client. Defaults to the Resource Principal home region.
const CursorRegion = "CURSOR_REGION"
