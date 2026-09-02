package resource

import "strings"

// ocidPrefix is what every real OCI resource identifier starts with; used to reject
// look-alike values at a path we'd otherwise treat as an OCID candidate (e.g. API Gateway's
// "source": "v1-deployment", or VCN flow logs' "source": "-").
const ocidPrefix = "ocid1."

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
	logContent, ok := logData["logContent"].(map[string]interface{})
	if !ok {
		return Extraction{}
	}

	logType, _ := logContent["type"].(string)

	rule := matchRule(logType)
	if rule == nil {
		return Extraction{}
	}

	ocid := getOCID(logContent, rule.OCIDPath)

	var name string
	if rule.NamePath != "" {
		name, _ = getPath(logContent, rule.NamePath)
	}

	return Extraction{
		OCID:         ocid,
		ExistingName: name,
		NeedsResolve: ocid != "" && name == "",
	}
}

// getOCID reads a dot-path and returns it only if it looks like a real OCI identifier --
// guards against a rule's OCIDPath resolving to something present but not actually an OCID for
// a given record (e.g. a field that's usually populated but happens to be empty or "-").
func getOCID(m map[string]interface{}, path string) string {
	v, ok := getPath(m, path)
	if !ok || !strings.HasPrefix(v, ocidPrefix) {
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
