// Package metrics accumulates and forwards the forwarder.* custom metrics described in
// the OCI Log Forwarder observability design (Phase 2) directly from this function's
// own process, using the same New Relic client library already used to forward logs.
package metrics

import (
	"os"

	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// Tier represents how much custom-metric detail a customer has opted into.
type Tier int

// Tier values are ordered so a higher tier always includes everything in a lower tier.
const (
	TierNone Tier = iota
	TierBasic
	TierAdvanced
)

// ParseTier converts a FORWARDER_METRICS_TIER value into a Tier, defaulting to TierNone
// for anything unrecognized so a typo never silently upgrades a customer's billing tier.
func ParseTier(value string) Tier {
	switch value {
	case common.MetricsTierBasic:
		return TierBasic
	case common.MetricsTierAdvanced:
		return TierAdvanced
	default:
		return TierNone
	}
}

// CurrentTier reads the configured tier from the FORWARDER_METRICS_TIER environment variable.
func CurrentTier() Tier {
	return ParseTier(os.Getenv(common.MetricsTier))
}

// enabledFor reports whether a metric declared at tier t should be emitted under the
// customer's configured tier.
func (t Tier) enabledFor(configured Tier) bool {
	return configured != TierNone && configured >= t
}

// Metric name constants for every forwarder.* metric this package emits. Call sites pass
// these (instead of ad hoc string literals) so MetricsByTier below stays the single place
// that documents which tier each metric belongs to.
const (
	MetricInvocations         = "forwarder.invocations"
	MetricRecordsReceived     = "forwarder.records.received"
	MetricRecordsDelivered    = "forwarder.records.delivered"
	MetricRecordsDropped      = "forwarder.records.dropped"
	MetricDeliveryDuration    = "forwarder.delivery.duration"
	MetricPipelineLag         = "forwarder.pipeline.lag"
	MetricPipelineLagNegative = "forwarder.pipeline.lag.negative"
)

// MetricsByTier documents which forwarder.* metrics are declared at each tier, purely for
// visibility -- it is not consulted by Recorder.Count/Summary, which are gated by the Tier
// argument passed at the call site. Keyed by Tier (rather than by metric name) so browsing
// "what does TierBasic include" is a single lookup: MetricsByTier[TierBasic]. Keep this in
// sync with those call sites by hand; there is currently no static check enforcing it.
var MetricsByTier = map[Tier][]string{
	TierBasic: {
		MetricInvocations,
		MetricRecordsReceived,
		MetricRecordsDelivered,
		MetricRecordsDropped,
		MetricDeliveryDuration,
		MetricPipelineLag,
		MetricPipelineLagNegative,
	},
}
