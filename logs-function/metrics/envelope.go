package metrics

import "time"

// Envelope holds the timestamp OCI's Logging CloudEvents-style envelope carries on every log
// record: https://docs.oracle.com/en-us/iaas/Content/Logging/Reference/top_level_logging_format.htm
//
// Two envelope timestamps could stand in for pipeline lag, and they are not interchangeable:
//
//   - oracle.ingestedtime: when the OCI Logging service received the record. Stamped by OCI's
//     own clock, so now()-ingestedtime measures how long the forwarder pipeline took to pick
//     the record up and deliver it -- which is exactly what forwarder.pipeline.lag should mean.
//   - time: the event time, supplied by the log *source*. It reflects the source
//     application's clock, which can run ahead of the forwarder host and yields
//     physically-impossible negative lag. We only fall back to it when ingestedtime is absent
//     (OCI itself defaults time to the ingestion time when a source omits it).
//
// The compartment/log-group/source-type dimensions the observability doc suggests are deferred
// to a follow-up PR alongside the advanced metric tier.
type Envelope struct {
	// LagTime is the timestamp forwarder.pipeline.lag is measured against: oracle.ingestedtime
	// when present, otherwise the source-supplied top-level time.
	LagTime time.Time
	// HasLagTime reports whether a usable lag-reference timestamp was found.
	HasLagTime bool
}

// ExtractEnvelope defensively pulls the lag-reference timestamp out of a raw log record. It
// prefers oracle.ingestedtime (OCI ingestion time, the correct anchor for pipeline latency)
// and falls back to the top-level time (source event time) only when ingestion time is
// unavailable. Real-world records vary in shape, so a missing or unexpected field is left at
// its zero value rather than causing an error.
func ExtractEnvelope(record map[string]interface{}) Envelope {
	var env Envelope
	if record == nil {
		return env
	}

	// Prefer OCI's ingestion time: it is set by the Logging service's clock, so it measures
	// forwarder pipeline latency without inheriting the log source's clock skew.
	if oracle, ok := record["oracle"].(map[string]interface{}); ok {
		if v, ok := oracle["ingestedtime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				env.LagTime = t
				env.HasLagTime = true
				return env
			}
		}
	}

	// Fall back to the source-supplied event time when ingestion time is unavailable.
	if v, ok := record["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			env.LagTime = t
			env.HasLagTime = true
		}
	}

	return env
}
