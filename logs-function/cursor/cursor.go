// Package cursor drops OCI log records already ingested during a Service
// Connector re-read window. After the reconciler modifies a connector, SCH
// re-delivers a rolling window of records; this keeps a per-log-group mark in
// Object Storage and drops anything already covered by it.
//
// POC: bare-minimum logic. Fails open (keeps records on any error).
package cursor

import (
	"time"
)

// Mark is the per-log-group high-water mark persisted in a Store. It combines a
// timestamp with the ids seen at that exact timestamp, so records delivered at
// the same instant as the mark can still be deduped individually.
type Mark struct {
	LastTimeUnixNano int64    `json:"lastTimeUnixNano"`
	SeenIDs          []string `json:"seenIds,omitempty"`
}

// record is a single log record reduced to what filter needs to make a
// keep/drop decision.
type record struct {
	id       string
	timeNano int64
	hasTime  bool
	idx      int
	raw      map[string]interface{}
}

// filter partitions recs into kept/dropped against mark and returns the mark
// advanced to reflect this batch. Records without a resolvable timestamp are
// always kept and never affect the mark. Ties at the mark's own timestamp are
// broken by SeenIDs so re-delivered records with a duplicate id are dropped
// while genuinely new records at the same instant are kept.
func filter(recs []record, mark Mark) (kept []record, updated Mark, dropped int) {
	newMax := mark.LastTimeUnixNano
	for _, r := range recs {
		if r.hasTime && r.timeNano > newMax {
			newMax = r.timeNano
		}
	}

	seenAtMax := make(map[string]struct{})
	if newMax == mark.LastTimeUnixNano {
		// Max didn't advance: boundary ties accumulate against the existing set.
		for _, id := range mark.SeenIDs {
			seenAtMax[id] = struct{}{}
		}
	}

	for _, r := range recs {
		switch {
		case !r.hasTime:
			kept = append(kept, r)
		case r.timeNano < mark.LastTimeUnixNano:
			dropped++
		case r.timeNano == mark.LastTimeUnixNano:
			if _, seen := seenAtMax[r.id]; seen {
				dropped++
			} else {
				kept = append(kept, r)
			}
			if r.timeNano == newMax {
				seenAtMax[r.id] = struct{}{}
			}
		default: // r.timeNano > mark.LastTimeUnixNano
			kept = append(kept, r)
			if r.timeNano == newMax {
				seenAtMax[r.id] = struct{}{}
			}
		}
	}

	updated.LastTimeUnixNano = newMax
	for id := range seenAtMax {
		updated.SeenIDs = append(updated.SeenIDs, id)
	}
	return kept, updated, dropped
}

// parseRecordTime resolves a record's timestamp from either an RFC3339Nano
// "time" string or an epoch-millisecond "datetime" number.
func parseRecordTime(r map[string]interface{}) (int64, bool) {
	if v, ok := r["time"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t.UnixNano(), true
		}
	}
	if v, ok := r["datetime"].(float64); ok {
		return int64(v) * int64(time.Millisecond), true
	}
	return 0, false
}

// recordID extracts the record's unique id, if present.
func recordID(r map[string]interface{}) string {
	if v, ok := r["id"].(string); ok {
		return v
	}
	return ""
}

// logGroupID extracts the OCID of the log group that produced the record.
func logGroupID(r map[string]interface{}) string {
	if o, ok := r["oracle"].(map[string]interface{}); ok {
		if v, ok := o["loggroupid"].(string); ok {
			return v
		}
	}
	return ""
}

// objectName is the Object Storage object name for a log group's mark.
func objectName(prefix, logGroupID string) string {
	return prefix + "cursor/" + logGroupID + ".json"
}
