// Package resource extracts a resource's OCID (and, when the payload already carries one, its
// display name) from a raw OCI log record, and drives resolving the ones missing a name via
// OCI Resource Search -- without ever calling that API from inside the per-record batching loop.
// See enrich.go for the collect-then-resolve-then-inject orchestration this package exists for.
package resource

import "strings"

// Rule declares, for one OCI log type (or type prefix), where in a record to find the resource's
// OCID and, when that type sometimes carries a display name in the payload, where to find that
// instead of resolving it via Resource Search. Paths are dot-separated and relative to the record
// itself (e.g. "data.gatewayId" means record.data.gatewayId) -- every OCI log type nests
// data/oracle/source directly at the top level.
type Rule struct {
	// Type is matched against a record's own type field -- exactly, unless PrefixMatch is set.
	Type string
	// PrefixMatch treats Type as a prefix instead of requiring an exact match. Used for log
	// sources that emit multiple type suffixes sharing the same OCID/name field layout (e.g.
	// Integration Cloud's activity-stream event types).
	PrefixMatch bool
	// OCIDPath locates the resource's OCID.
	OCIDPath string
	// NamePath locates an existing display name in the payload, for a type where the name is
	// present on some but not necessarily all records. Empty means this type never carries a
	// name anywhere, so a match always requires a Resource Search lookup. Do NOT add a rule (or
	// a NamePath) for a type whose name is *always* present -- entity synthesis reads that
	// native field directly with zero involvement from this package, so extracting/injecting it
	// here would be dead weight.
	// This table exists only for types where synthesis has nothing to go on without us: API
	// Gateway, Service Connector Hub, Functions, Load Balancer, Object Storage, Queue, and
	// Streaming were all confirmed (via their real entity-synthesis rules, or explicit
	// confirmation) to always carry a name, and deliberately have no rule here for that reason.
	NamePath string
}

// rules is the only source of truth for which log types this package touches at all. A record
// whose type matches nothing here gets no OCID, no name, no injected fields, and never reaches
// Resource Search -- deliberately, not as a fallback-of-last-resort: we only ever attempt a
// lookup for a type we've explicitly verified has an OCID worth resolving. Extending coverage
// to a new type means adding a Rule here, not relying on a generic guess that might land on the
// wrong field, or resolve something never meant to be resolved, for a type nobody's checked.
var rules = []Rule{
	{
		// Exact type seen: com.oraclecloud.networkfirewall.traffic. Prefix so any sibling
		// Network Firewall log type also gets a shot at data.firewall-id. No name anywhere in
		// the payload for this type.
		Type:        "com.oraclecloud.networkfirewall.",
		PrefixMatch: true,
		OCIDPath:    "data.firewall-id",
	},
	{
		// Exact type seen: com.oraclecloud.dns.private.resolver. Prefix so a sibling private-DNS
		// log type also gets a shot at source. No name anywhere in the payload for this type.
		Type:        "com.oraclecloud.dns.private.",
		PrefixMatch: true,
		OCIDPath:    "source",
	},
	{
		// Covers every Integration Cloud activity-stream event type sharing this prefix
		// (Receive, Send, etc.), all identifying the same integration instance the same way.
		// No name anywhere in the payload for this type.
		Type:        "com.oraclecloud.integration.integrationinstance.",
		PrefixMatch: true,
		OCIDPath:    "source",
	},
	{
		// Events Service rule-execution logs identify the matched rule via data.ruleId, not
		// data.resourceId like every _Audit-style event -- no generic path can guess this field
		// name, so it needs an explicit rule. Prefix (still lowercase "eventsservice", so this
		// stays distinct from the differently-cased com.oraclecloud.EventsService.* audit type,
		// which is a separate, case-sensitive namespace) so a sibling eventrule log type also
		// gets a shot at data.ruleId. No NamePath declared: unlike API Gateway/SCH/Functions/etc.,
		// we don't have this type's own entity-synthesis rule to confirm source
		// ("poc-reconciler-rule" in the one sample seen) is *always* populated, so it's treated
		// as needing a resolve rather than assumed safe to skip.
		Type:        "com.oraclecloud.EventsService.",
		PrefixMatch: true,
		OCIDPath:    "data.ruleId",
	},
	{
		// source duplicates the same OCID here (not a name) -- no NamePath, this genuinely
		// needs a Resource Search resolve.
		Type:        "com.oraclecloud.goldengate.",
		PrefixMatch: true,
		OCIDPath:    "data.resourceId",
	},
}

// matchRule returns the first rule whose Type matches logType, or nil if none do -- callers
// must treat nil as "this record is out of scope for this package," not as a signal to guess at
// some other field. See rules' own doc comment for why there is no fallback path here.
func matchRule(logType string) *Rule {
	for i := range rules {
		r := &rules[i]
		if r.PrefixMatch {
			if strings.HasPrefix(logType, r.Type) {
				return r
			}
			continue
		}
		if logType == r.Type {
			return r
		}
	}
	return nil
}
