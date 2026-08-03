# SCH Auto-Subscribe Reconciler (POC)

An OCI Function that keeps a **Service Connector Hub (SCH)** connector's log-source
list in sync with the log groups in a tenancy, so new log groups are automatically
forwarded to New Relic and deleted ones are removed — without editing Terraform.

This is the reconciler half of the auto-subscribe design (Option H in the DACI).
The other half is the forwarder-side **cursor** dedup (see `../logs-function/cursor`),
which drops records already ingested during the brief re-ingest window that follows
an `UpdateServiceConnector` call.

## How it works

1. An **OCI Events rule** (scoped to the root compartment so it sees the whole
   tenancy) matches log-group lifecycle events from the `loggingService` source and
   invokes this function.
2. The function parses the CloudEvent, classifies it as add (create/update) or
   remove (delete), reads the connector's current log-sources via
   `GetServiceConnector`, computes the desired set, and — only if it changed —
   writes it back via `UpdateServiceConnector`.
3. The operation is **idempotent**: a duplicate create or a delete of an unknown
   group makes no SCH call. OCI Events delivery is at-least-once, so this matters.

## Configuration (environment variables)

| Var                      | Required | Purpose                                             |
| ------------------------ | -------- | --------------------------------------------------- |
| `SERVICE_CONNECTOR_OCID` | yes      | OCID of the SCH connector to keep in sync           |
| `CONNECTOR_REGION`       | no       | Pin the SCH client to a region (else RP home region)|
| `DEBUG_ENABLED`          | no       | `"true"` for debug logging                          |

Auth is **Resource Principal**, matching the forwarder (see
`../logs-function/util/secrets_util.go`).

## Required IAM

The function's dynamic group needs to read and update the target connector, e.g.:

```
allow dynamic-group <reconciler-dg> to use serviceconnectors in tenancy where target.serviceConnector.id = '<SERVICE_CONNECTOR_OCID>'
allow dynamic-group <reconciler-dg> to read log-groups in tenancy
```

Tighten to a compartment if a tenancy-wide grant is not acceptable.

## Events rule

Match log-group create/delete (and optionally update) from `loggingService`.
The event-type strings must be **pinned to what the target tenancy actually emits** —
`classify()` matches on the lower-cased suffix (`createloggroup` / `deleteloggroup` /
`updateloggroup`) to stay tolerant, but the rule itself should filter precisely so the
function is not invoked for unrelated events.

## Terraform ownership

Because the reconciler mutates `source.log_sources` at runtime, Terraform must **not**
fight it. Add to the connector resource:

```hcl
lifecycle {
  ignore_changes = [source[0].log_sources]
}
```

Terraform still owns the connector's existence, target, and tasks; the reconciler owns
membership.

## Build & test

No Go toolchain was available where this POC was authored, so it has **not** been
compiled here. Before deploying:

```bash
cd reconciler
go mod tidy      # generates go.sum
go vet ./...
go test ./...    # unit tests cover add/remove/idempotency/limit — no live tenancy needed
```

The SDK is isolated behind the `ConnectorManager` interface, so the reconcile logic is
tested with an in-memory fake (`reconcile_test.go`). Verify the `sch` SDK type/method
names (`LoggingSourceDetails`, `UpdateLoggingSourceDetails`, `LogSource`) against
`oci-go-sdk/v65` when you run `go vet` — they are written from the documented API.

## Known limitations (POC)

- **Concurrency:** two events processed at once could race on read-modify-write of the
  connector. SCH has no conditional update; for the POC, event volume is low and OCI
  serializes connector updates server-side, but a production version should add a
  work-queue or per-connector lock.
- **Bootstrap / drift:** this function only reacts to events. Initial population and
  periodic full reconciliation (design items #5/#6) are not implemented here.
- **Sharding:** refuses to exceed `maxLogSourcesPerConnector`; multi-connector sharding
  is not implemented.
