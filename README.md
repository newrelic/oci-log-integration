[![Community Plus header](https://github.com/newrelic/opensource-website/raw/master/src/images/categories/Community_Plus.png)](https://opensource.newrelic.com/oss-category/#community-plus)

# New Relic OCI Log Integrations

This repository contains integrations to forward logs from Oracle Cloud Infrastructure (OCI).

## Prerequisites

* [New Relic Ingest Key & API Key](https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/#license-key)
* OCI user with Cloud Administrator role to create resources/stacks

## Custom forwarder metrics

The log forwarder Function can emit its own custom metrics directly to New Relic's Metric API, in addition to forwarding logs. This is controlled via the `metrics_tier` Terraform variable (`none` / `basic` / `advanced`; default `basic`). `basic` covers core health (record counts, delivery success/loss, delivery latency, pipeline lag); `advanced` adds deeper root-cause/tuning metrics (byte volumes, decode/serialize errors, batching behavior, delivery error classes, run duration, secret-fetch failures, client-cache hit rate) on top of everything in `basic`.

## Image hosting and versioning

The log-forwarder function image is published to Docker Hub (`docker.io/newrelic/oci-log-forwarder`). At apply time, the stack copies it into a private Container Registry repository in your own tenancy and runs the Function from that copy — your tenancy never pulls directly from Docker Hub at runtime, only during `apply`.

This means **the machine running `apply` needs outbound network access to Docker Hub** (`registry-1.docker.io` and `auth.docker.io`), in addition to the OCI API endpoints the stack already requires. If your environment restricts outbound internet access, allow-list those hosts before applying.

**Versioning:** the `function_image` variable defaults to the `:latest` tag. Re-applying the stack checks what image that tag currently points to and copies the new one if it's changed, so you'll pick up new releases automatically on your next `apply` without any action. To control exactly when you upgrade instead, pin `function_image` to a specific version tag (e.g. `docker.io/newrelic/oci-log-forwarder:1.42`) — the stack will then only ever copy that pinned version, and upgrading becomes a deliberate change to that variable followed by `apply`.

## Contributing

We encourage your contributions to improve oci-log-integration! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project. If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company, please drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

## License

oci-log-integration is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
