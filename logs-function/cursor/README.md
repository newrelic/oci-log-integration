# cursor — per-log-group dedup for the forwarder

When the reconciler adds or removes a log source on the Service Connector, OCI
Service Connector Hub re-reads a rolling window (~24h) of each affected log group
and **re-delivers** those records to this forwarder. Without dedup, those records
are ingested a second time at New Relic. This package drops the already-seen
records using a per-log-group high-water mark persisted in OCI Object Storage.

## How it works

For each log group in a batch, the forwarder loads a `Mark` from Object Storage,
drops records at or below it, then writes the advanced mark back.

The mark is a hybrid of `(timestamp, id)`:

- record timestamp **<** mark → drop (already ingested)
- record timestamp **>** mark → keep
- record timestamp **==** mark → keep only if the record `id` was not already
  seen at that timestamp

The tie-break on `id` matters because many OCI records can share the same
millisecond; a timestamp-only cursor would drop distinct records that happen to
land on the boundary. Timestamps come from the record `time` field
(RFC3339/RFC3339Nano), falling back to `datetime` (epoch millis). Log-group
attribution comes from `oracle.loggroupid`.

State is stored at `<prefix>cursor/<logGroupOCID>.json`. Concurrent writers are
handled with conditional PUTs (`If-Match` on the loaded ETag, or `If-None-Match: *`
for a first write); a `412 Precondition Failed` means another invocation advanced
the cursor first and is treated as benign.

## Fail-open

Dedup is best-effort by design — a few duplicates are preferable to dropped logs.
Records are forwarded unchanged whenever:

- `CURSOR_ENABLED` is not exactly `"true"` (the default — OFF),
- `CURSOR_NAMESPACE` or `CURSOR_BUCKET` is unset,
- the Object Storage client cannot be created,
- a mark cannot be loaded for a group, or
- a record has no resolvable log-group id or timestamp.

A failed *save* does not un-drop records already filtered in that invocation; the
next re-ingest simply re-filters against the last durably persisted mark.

## Environment variables

| Variable            | Required | Description                                                        |
|---------------------|----------|--------------------------------------------------------------------|
| `CURSOR_ENABLED`    | no       | `"true"` to enable dedup. Anything else = OFF (default).           |
| `CURSOR_NAMESPACE`  | when on  | Object Storage namespace holding cursor state.                     |
| `CURSOR_BUCKET`     | when on  | Object Storage bucket holding cursor state.                        |
| `CURSOR_PREFIX`     | no       | Object-name prefix, e.g. `nr-logs/`. Default empty.                |
| `CURSOR_REGION`     | no       | Region override for the Object Storage client. Default: RP home.   |

## Required IAM

The function's dynamic group needs read/write on the cursor bucket's objects:

```
Allow dynamic-group <fn-dg> to manage objects in compartment <c> where target.bucket.name = '<CURSOR_BUCKET>'
```

`manage objects` covers the GET/PUT (with conditional headers) this package uses.
Scope to the single bucket as shown rather than granting tenancy-wide.

## Rollout

The forwarder calls dedup behind a flag check at the top of the OCI_LOGGING path
(`applyCursorDedup` in `logs-function/main.go`); the existing forward path is
untouched. Ship with `CURSOR_ENABLED` unset, create the bucket + IAM, then flip
the flag to `"true"`. Rollback = unset the flag (no redeploy of logic required).

## Build & test

Run from `logs-function/` in a real Go toolchain (the package uses only
`oci-go-sdk/v65` subpackages already required by the module — no new dependency):

```
gofmt -l cursor/
go vet ./cursor/...
go test ./cursor/...
```

## Known limitations

- **Not exactly-once.** Dedup is a best-effort filter over an at-least-once
  pipeline; treat New Relic-side idempotency as the real guarantee.
- **Concurrent invocations** on the same log group race on the mark; the
  conditional PUT keeps state consistent but the loser's advance is dropped, so a
  small number of duplicates can slip through under high fan-out.
- **Marks grow unbounded in count** (one small object per log group); prune
  objects for deleted log groups out of band if needed.
- **Clock/format assumptions:** records missing both `time` and `datetime` are
  always kept (fail-open) and never advance the mark.
