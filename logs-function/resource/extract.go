package resource

import (
	"strings"

	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// Extraction is what Extract found for one log record: the OCID to associate the log with (if
// any), a display name already present in the payload for that OCID (if any), and whether a
// resolve via Resource Search is needed at all.
type Extraction struct {
	OCID         string
	ExistingName string
	NeedsResolve bool
}

// Extract inspects one raw OCI log record and determines its OCID and, when available, its
// name -- without ever calling Resource Search itself; that's the caller's job (see enrich.go),
// driven by NeedsResolve. A record whose type doesn't match any Rule in rules.go yields a zero
// Extraction (nothing found, nothing to resolve) -- see rules' own doc comment for why there is
// no fallback guess for an unmatched type.
func Extract(logData map[string]interface{}) Extraction {
	// A record is the CloudEvent itself -- type/data/source/oracle directly at the top level, as
	// Service Connector Hub actually delivers it. No wrapper to unwrap.
	logType, _ := logData["type"].(string)

	rule := matchRule(logType)
	if rule == nil {
		return Extraction{}
	}

	ocid := getOCID(logData, rule.OCIDPath, logType)
	if ocid == "" {
		return Extraction{}
	}

	var name string
	if rule.NamePath != "" {
		name, _ = getPath(logData, rule.NamePath)
	}

	return Extraction{
		OCID:         ocid,
		ExistingName: name,
		NeedsResolve: name == "",
	}
}

// getOCID reads a dot-path and returns it only if it looks like a real OCI identifier --
// guards against a rule's OCIDPath resolving to something present but not actually an OCID for
// a given record (e.g. a field that's usually populated but happens to be empty or "-"). logType
// is used only for the warning below, identifying which rule's path came up empty without
// logging the record's own field value (customer log content).
func getOCID(m map[string]interface{}, path string, logType string) string {
	v, ok := getPath(m, path)
	if !ok {
		log.Warnf("resource OCID extraction: type %q matched a rule but path %q was not found", logType, path)
		return ""
	}
	if !strings.HasPrefix(v, common.OCIDPrefix) {
		log.Warnf("resource OCID extraction: type %q matched a rule but value at path %q does not look like an OCID", logType, path)
		return ""
	}
	return v
}

// getPath walks a dot-separated path (e.g. "data.gatewayId") through nested
// map[string]interface{} values, relative to m. Returns ok=false for a missing key, a non-map
// intermediate, or a non-string (including JSON null) leaf value -- every case is "nothing
// found here," not an error, since payload shape varies legitimately across OCI log types.
func getPath(m map[string]interface{}, path string) (string, bool) {
	keys := strings.Split(path, ".")
	var cur interface{} = m
	for _, key := range keys {
		asMap, ok := cur.(map[string]interface{})
		if !ok {
			return "", false
		}
		cur, ok = asMap[key]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}
