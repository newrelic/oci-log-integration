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

// ResourceSearchErrorTTL is the environment variable name for how long, in seconds, a failed
// Resource Search client creation is cached before the next call retries it. Kept shorter than
// ClientTTL by default so a transient Resource Principal auth failure (e.g. on a cold start)
// doesn't leave the OCID -> name enrichment feature dark for the full success TTL window.
const ResourceSearchErrorTTL = "RESOURCE_SEARCH_ERROR_TTL"

// DefaultResourceSearchErrorTTL is the default for ResourceSearchErrorTTL, in seconds (30 seconds).
const DefaultResourceSearchErrorTTL = 30
// NegativeCacheTTLSeconds bounds how long a failed license-key fetch or Metrics client
// creation is cached before being retried, in seconds. Kept much shorter than
// ClientTTL/DefaultClientTTL so a transient Vault blip or config error recovers quickly
// instead of being replayed as a cached failure for the full success-case TTL.
const NegativeCacheTTLSeconds = 30

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

// ResourceResolveTimeoutSeconds is the environment variable name for how long, in seconds, a
// single invocation's resolve-many phase may run before it's abandoned in favor of shipping
// logs without a resolved name rather than blocking forwarding.
const ResourceResolveTimeoutSeconds = "RESOURCE_RESOLVE_TIMEOUT_SECONDS"

// DefaultResourceResolveTimeoutSeconds is the default for ResourceResolveTimeoutSeconds.
const DefaultResourceResolveTimeoutSeconds = 10

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

// NameFieldKey is a synthetic field added to capture the resource's name in the OCI payload.
const NameFieldKey = "logging.oci.displayName"

// OCI_LOGGING represents the event type for Oracle Cloud Infrastructure logging events.
const OCI_LOGGING = "ociLogging"

// OCIDPrefix is what every real OCI resource identifier starts with; used to reject look-alike
// values at a path we'd otherwise treat as an OCID candidate.
const OCIDPrefix = "ocid1."

// OCIDNameCacheTTL is the environment variable name for how long, in seconds, a resolved
// OCID -> display name result (including a confirmed "no name found") is reused before
// Resource Search is asked again for that OCID. This is what makes repeated invocations on a
// warm container cheap: a Service Connector Hub fires this function repeatedly for the same
// small set of resources, so without this cache every invocation would re-pay a Resource Search
// round trip for names already resolved seconds/a batch earlier.
const OCIDNameCacheTTL = "OCID_NAME_CACHE_TTL"

// DefaultOCIDNameCacheTTL is the default for OCIDNameCacheTTL, in seconds (5 minutes).
const DefaultOCIDNameCacheTTL = 300
// MetricsTier is the name of the environment variable that selects which tier of
// custom forwarder.* metrics is emitted (none/basic/advanced).
const MetricsTier = "FORWARDER_METRICS_TIER"

// Metrics tier values accepted by MetricsTier.
const (
	MetricsTierNone     = "none"
	MetricsTierBasic    = "basic"
	MetricsTierAdvanced = "advanced"
)

// FunctionNameEnvVar is the environment variable the Fn/OCI Functions runtime injects
// automatically at invocation time with the function's name.
const FunctionNameEnvVar = "FN_FN_NAME"

// ApplicationNameEnvVar is the environment variable the Fn/OCI Functions runtime injects
// automatically at invocation time with the function's parent Application's name.
const ApplicationNameEnvVar = "FN_APP_NAME"

// TenancyName is the environment variable name for the OCI tenancy's display name, set by
// Terraform (from data.oci_identity_tenancy) so multiple forwarders reporting into one New
// Relic account can be told apart by tenancy without an extra per-invocation API call.
const TenancyName = "TENANCY_NAME"

// CompartmentName is the environment variable name for the OCI compartment's display name
// (not OCID -- kept human-readable and bounded), set by Terraform for the same reason as
// TenancyName.
const CompartmentName = "COMPARTMENT_NAME"
