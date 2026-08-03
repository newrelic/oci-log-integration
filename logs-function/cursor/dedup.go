package cursor

import (
	"context"

	"github.com/sirupsen/logrus"
)

// Apply removes records that have already been ingested, using the per-log-group
// marks in the given Store. It is fail-open: any error loading or saving a mark
// leaves that group's records untouched (a few duplicates are preferable to
// dropped logs). Input order is preserved. Records without a resolvable log-group
// id or timestamp are always kept.
//
// The parameter and return types are []map[string]interface{}, which is the
// underlying type of common.OCILoggingEvent, so callers can pass the event
// directly.
func Apply(ctx context.Context, store Store, log *logrus.Logger, records []map[string]interface{}) []map[string]interface{} {
	if len(records) == 0 || store == nil {
		return records
	}

	// Group record indices by owning log group, preserving first-seen order.
	groups := make(map[string][]int)
	var order []string
	for i, raw := range records {
		lg := logGroupID(raw)
		if lg == "" {
			continue // cannot attribute to a group; always kept
		}
		if _, ok := groups[lg]; !ok {
			order = append(order, lg)
		}
		groups[lg] = append(groups[lg], i)
	}

	drop := make([]bool, len(records))
	totalDropped := 0

	for _, lg := range order {
		idxs := groups[lg]
		recs := make([]record, len(idxs))
		for j, i := range idxs {
			raw := records[i]
			tn, ok := parseRecordTime(raw)
			recs[j] = record{id: recordID(raw), timeNano: tn, hasTime: ok, idx: i, raw: raw}
		}

		mark, etag, _, err := store.Load(ctx, lg)
		if err != nil {
			log.WithField("logGroupId", lg).Warnf("cursor load failed, keeping all records for this group: %v", err)
			continue
		}

		kept, updated, dropped := filter(recs, mark)
		if dropped == 0 {
			// Even with nothing dropped the mark may need to advance so future
			// re-ingests are deduped; persist it.
			if err := store.Save(ctx, lg, updated, etag); err != nil {
				logSaveErr(log, lg, err)
			}
			continue
		}

		keptIdx := make(map[int]struct{}, len(kept))
		for _, r := range kept {
			keptIdx[r.idx] = struct{}{}
		}
		for _, i := range idxs {
			if _, ok := keptIdx[i]; !ok {
				drop[i] = true
			}
		}
		totalDropped += dropped

		if err := store.Save(ctx, lg, updated, etag); err != nil {
			logSaveErr(log, lg, err)
		}
	}

	if totalDropped == 0 {
		return records
	}

	out := make([]map[string]interface{}, 0, len(records)-totalDropped)
	for i, raw := range records {
		if !drop[i] {
			out = append(out, raw)
		}
	}
	log.Infof("cursor dedup dropped %d/%d already-ingested records", totalDropped, len(records))
	return out
}

func logSaveErr(log *logrus.Logger, lg string, err error) {
	if IsPreconditionFailed(err) {
		// Another invocation advanced the cursor first; benign under at-least-once
		// re-delivery. Our filtered output still stands.
		log.WithField("logGroupId", lg).Info("cursor save skipped: concurrent update won (precondition failed)")
		return
	}
	log.WithField("logGroupId", lg).Warnf("cursor save failed (dedup best-effort): %v", err)
}
