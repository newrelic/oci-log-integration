[![Community Plus header](https://github.com/newrelic/opensource-website/raw/master/src/images/categories/Community_Plus.png)](https://opensource.newrelic.com/oss-category/#community-plus)

# New Relic OCI Log Integrations

This repository contains integrations to forward logs from Oracle Cloud Infrastructure (OCI).

## Prerequisites

* [New Relic Ingest Key & API Key](https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/#license-key)
* OCI user with Cloud Administrator role to create resources/stacks

## Custom forwarder metrics

The log forwarder Function can emit its own custom metrics directly to New Relic's Metric API, in addition to forwarding logs. This is controlled via the `metrics_tier` Terraform variable (`none` / `basic` / `advanced`; default `basic`). `basic` covers core health (record counts, delivery success/loss, delivery latency, pipeline lag); `advanced` adds deeper root-cause/tuning metrics (byte volumes, decode/serialize errors, batching behavior, delivery error classes, run duration, secret-fetch failures, client-cache hit rate) on top of everything in `basic`.

## Entity synthesis name enrichment

The log forwarder resolves the OCID embedded in a log record to its human-readable OCI display name via OCI Resource Search, and injects it as `logging.oci.displayName` on the record. This feeds entity synthesis downstream — synthesized OCI entities get a real name instead of a raw OCID.

Currently covers 4 log types via prefix-matched rules: Network Firewall, DNS Private Resolver, Integration, and GoldenGate. There's deliberately no rule for Queue, Streaming, or API Gateway — their logs already carry a native display name in the payload, so a Resource Search lookup would be redundant. This is always on; there's no opt-out variable.

Resolution is batched per invocation: every record is scanned once, the distinct OCIDs needing a lookup are deduped across the whole batch, chunked to Resource Search's 20-OCID-per-query limit, and resolved concurrently with retry on transient failures. A partial failure doesn't block forwarding — unresolved records just go out without a name.

## Contributing

We encourage your contributions to improve oci-log-integration! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project. If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company, please drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

## License

oci-log-integration is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
