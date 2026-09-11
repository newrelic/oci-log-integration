package resource

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
	"github.com/sirupsen/logrus"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// Resolver resolves a set of OCIDs to their display names in one call. Implementations are
// responsible for chunking to whatever limit the underlying API imposes (see
// common.MaxIdentifiersPerQuery) and for running chunks concurrently -- EnrichRecords calls this
// exactly once per invocation, with the full deduped set of OCIDs Extract flagged as needing one.
type Resolver interface {
	ResolveMany(ctx context.Context, ocids []string) (map[string]string, error)
}

// EnrichRecords scans records once, resolves every distinct OCID that Extract flagged as
// NeedsResolve in a single bounded call to resolver, and writes logging.oci.displayName back
// into each record that ended up with a name. Never called at all when the feature flag is off
// -- that decision lives in main.go, not here, so this function has no flag check of its own.
func EnrichRecords(ctx context.Context, records common.OCILoggingEvent, resolver Resolver) common.OCILoggingEvent {
	logMemStats("enrichment start")

	extractions := make([]Extraction, len(records))
	pending := map[string]struct{}{}

	for i, rec := range records {
		ext := Extract(rec)
		extractions[i] = ext
		if ext.NeedsResolve {
			pending[ext.OCID] = struct{}{}
		}
	}

	log.Debugf("resource name enrichment: %d record(s) scanned, %d distinct OCID(s) need resolution", len(records), len(pending))

	if len(pending) > 0 && resolver != nil {
		ocids := make([]string, 0, len(pending))
		for ocid := range pending {
			ocids = append(ocids, ocid)
		}

		resolveCtx, cancel := context.WithTimeout(ctx, resolveTimeout())
		resolved, err := resolver.ResolveMany(resolveCtx, ocids)
		cancel()
		if err != nil {
			log.Warnf("resource name resolution incomplete, some logs will ship without a name: %v", err)
		}
		log.Debugf("resource name enrichment: %d of %d distinct OCID(s) resolved", len(resolved), len(ocids))

		for i := range extractions {
			if !extractions[i].NeedsResolve {
				continue
			}
			if name := resolved[extractions[i].OCID]; name != "" {
				extractions[i].ExistingName = name
			}
		}
	}

	for i, rec := range records {
		injectResourceName(rec, extractions[i].ExistingName)
	}

	logResolvedNames(extractions)

	logMemStats("enrichment end")

	return records
}

// logMemStats logs a heap/GC snapshot at debug level only -- runtime.ReadMemStats itself isn't
// free (it briefly stops the world on some Go versions to get a consistent snapshot), so this
// skips the call entirely rather than gathering stats nobody's going to see.
func logMemStats(point string) {
	if !log.IsLevelEnabled(logrus.DebugLevel) {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.WithFields(logrus.Fields{
		"point":       point,
		"heapAllocMB": m.HeapAlloc / 1024 / 1024,
		"sysMB":       m.Sys / 1024 / 1024,
		"numGC":       m.NumGC,
	}).Debug("resource name enrichment memory footprint")
}

// logResolvedNames logs each record's OCID and resolved name at debug level -- a lightweight aid
// for verifying what Resource Search actually resolved, since the function's HTTP response body
// is always empty. One line per record, so this needs no separate opt-in beyond DebugEnabled --
// unlike a full-payload dump, it's cheap enough to always emit alongside the rest of debug
// logging. Deliberately logs only the OCID/name pair, not the surrounding log content, so this
// stays safe to enable without writing the customer's real OCI log content into this function's
// own execution logs.
func logResolvedNames(extractions []Extraction) {
	if !log.IsLevelEnabled(logrus.DebugLevel) {
		return
	}
	for i, ext := range extractions {
		if ext.OCID == "" {
			continue
		}
		name := ext.ExistingName
		if name == "" {
			name = "<unresolved>"
		}
		log.Debugf("resource name enrichment: record %d ocid=%s name=%s", i, ext.OCID, name)
	}
}

// resolveTimeout bounds how long the one resolve-many call per invocation may run, via
// common.ResourceResolveTimeoutSeconds (default common.DefaultResourceResolveTimeoutSeconds).
// On expiry, resolver.ResolveMany is expected to return whatever it resolved so far rather than
// nothing -- the remaining records simply ship without a name instead of blocking forwarding.
func resolveTimeout() time.Duration {
	seconds := common.DefaultResourceResolveTimeoutSeconds
	if v := os.Getenv(common.ResourceResolveTimeoutSeconds); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			seconds = parsed
		}
	}
	return time.Duration(seconds) * time.Second
}

// nameFieldKey is a literal attribute key (dots included, not a nested path) matching the
// "logging."-namespaced convention for a log-derived version of the oci.displayName golden tag
// entity synthesis rules already use (see rules.go's YAML-derived rules for oci.displayName
// appearing as a goldenTag).
const nameFieldKey = "logging.oci.displayName"

// injectResourceName writes nameFieldKey into the record's data object. A no-op when there's no
// name.
func injectResourceName(logData map[string]interface{}, name string) {
	if name == "" {
		return
	}

	data, ok := logData["data"].(map[string]interface{})
	if !ok {
		data = map[string]interface{}{}
		logData["data"] = data
	}

	data[nameFieldKey] = name
}
