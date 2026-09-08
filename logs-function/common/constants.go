// Package common provides common constants structs and variables.
package common

import "time"

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

// MaxIdentifiersPerQuery is OCI Resource Search's hard count limit on the number of values
// allowed in an "identifier in (...)" structured-query clause. Verified empirically: a 20-item
// list succeeds, a 21-item list fails with CannotParseRequest at nearly the same query length,
// confirming this is a genuine count limit, not the separate (much larger) 50,000-character
// total-query-length cap.
const MaxIdentifiersPerQuery = 20

// ResourceSearchWorkerPool bounds how many chunked Resource Search calls run concurrently
// when resolving a batch's distinct missing-name OCIDs.
const ResourceSearchWorkerPool = 5

// ResourceNameEnrichmentEnabled is the environment variable name that gates the OCID -> resource
// name enrichment feature. When unset or not "true", none of the enrichment code path runs and
// log forwarding behaves exactly as it did before this feature existed.
const ResourceNameEnrichmentEnabled = "RESOURCE_NAME_ENRICHMENT_ENABLED"

// ResourceResolveTimeoutSeconds is the environment variable name for how long, in seconds, a
// single invocation's resolve-many phase may run before it's abandoned in favor of shipping
// logs without a resolved name rather than blocking forwarding.
const ResourceResolveTimeoutSeconds = "RESOURCE_RESOLVE_TIMEOUT_SECONDS"

// DefaultResourceResolveTimeoutSeconds is the default for ResourceResolveTimeoutSeconds.
const DefaultResourceResolveTimeoutSeconds = 8

// MaxSearchRetries is how many extra attempts a single Resource Search chunk gets after a
// retriable (429/5xx) failure, before giving up and letting that chunk's OCIDs ship without a
// name.
const MaxSearchRetries = 3

// BaseRetryDelay and MaxRetryDelay bound the exponential-backoff-with-full-jitter delay between
// Resource Search retry attempts: a random duration between 0 and
// min(MaxRetryDelay, BaseRetryDelay * 2^attempt).
const (
	BaseRetryDelay = 100 * time.Millisecond
	MaxRetryDelay  = 1500 * time.Millisecond
)

// LogTransformedPayloadEnabled is the environment variable name that gates logging the full
// post-enrichment payload as pretty-printed JSON. Off by default even when DebugEnabled is on --
// this is a separate, explicit opt-in for a verbose diagnostic aid, not general debug logging.
//
// Turning this on writes the customer's real OCI log content -- the whole post-enrichment batch,
// not just metadata -- into this function's own execution logs, which is a second place that
// data now lives. Only turn this on for as long as you're actively troubleshooting, in a
// non-production environment where that's acceptable; do not leave it on in production.
const LogTransformedPayloadEnabled = "LOG_TRANSFORMED_PAYLOAD_ENABLED"
