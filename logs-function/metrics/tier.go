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

// Metric bundles a forwarder.* metric's name together with the tier it belongs to, so a call
// site passes one identifier instead of separately supplying a tier that must be kept in sync
// with the metric by hand.
type Metric struct {
	Name string
	tier Tier
}

// enabledFor reports whether m should be emitted under the customer's configured tier.
func (m Metric) enabledFor(configured Tier) bool {
	return m.tier.enabledFor(configured)
}

// basic and advanced build a Metric for their respective tier, so Basic/Advanced below don't
// each repeat their own tier on every line -- which would let a copy-paste mistake put a
// TierAdvanced metric inside the Basic struct (or vice versa) without anything catching it.
func basic(name string) Metric    { return Metric{Name: name, tier: TierBasic} }
func advanced(name string) Metric { return Metric{Name: name, tier: TierAdvanced} }

// BasicMetrics groups every forwarder.* metric declared at TierBasic. Access them as
// metrics.Basic.MetricInvocations, etc. -- the namespace itself says which tier a metric
// belongs to, so a call site never needs to state the tier separately.
type BasicMetrics struct {
	MetricInvocations         Metric
	MetricRecordsReceived     Metric
	MetricRecordsDelivered    Metric
	MetricRecordsDropped      Metric
	MetricDeliveryDuration    Metric
	MetricPipelineLag         Metric
	MetricPipelineLagNegative Metric
}

// AdvancedMetrics groups every forwarder.* metric declared at TierAdvanced. Access them as
// metrics.Advanced.MetricBytesReceived, etc.
type AdvancedMetrics struct {
	MetricClientInitErrors Metric
	MetricRunDuration      Metric
	MetricDeliveryErrors   Metric
	MetricBytesDelivered   Metric
	MetricClientCache      Metric
	MetricBytesReceived    Metric
	MetricDecodeErrors     Metric
	MetricSerializeErrors  Metric
	MetricRecordsOversized Metric
	MetricBatchesCreated   Metric
	MetricBatchSizeBytes   Metric
}

// Basic holds every TierBasic metric this package emits.
var Basic = BasicMetrics{
	MetricInvocations:         basic("forwarder.invocations"),
	MetricRecordsReceived:     basic("forwarder.records.received"),
	MetricRecordsDelivered:    basic("forwarder.records.delivered"),
	MetricRecordsDropped:      basic("forwarder.records.dropped"),
	MetricDeliveryDuration:    basic("forwarder.delivery.duration"),
	MetricPipelineLag:         basic("forwarder.pipeline.lag"),
	MetricPipelineLagNegative: basic("forwarder.pipeline.lag.negative"),
}

// Advanced holds every TierAdvanced metric this package emits.
var Advanced = AdvancedMetrics{
	MetricClientInitErrors: advanced("forwarder.client.init.errors"),
	MetricRunDuration:      advanced("forwarder.run.duration"),
	MetricDeliveryErrors:   advanced("forwarder.delivery.errors"),
	MetricBytesDelivered:   advanced("forwarder.bytes.delivered"),
	MetricClientCache:      advanced("forwarder.client.cache"),
	MetricBytesReceived:    advanced("forwarder.bytes.received"),
	MetricDecodeErrors:     advanced("forwarder.decode.errors"),
	MetricSerializeErrors:  advanced("forwarder.serialize.errors"),
	MetricRecordsOversized: advanced("forwarder.records.oversized"),
	MetricBatchesCreated:   advanced("forwarder.batches.created"),
	MetricBatchSizeBytes:   advanced("forwarder.batch.size_bytes"),
}
