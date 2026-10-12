[![Community Plus header](https://github.com/newrelic/opensource-website/raw/master/src/images/categories/Community_Plus.png)](https://opensource.newrelic.com/oss-category/#community-plus)

# New Relic OCI Log Integrations

This repository contains integrations to forward logs from Oracle Cloud Infrastructure (OCI).

## Prerequisites

* [New Relic Ingest Key & API Key](https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/#license-key)
* OCI user with Cloud Administrator role to create resources/stacks

## Custom forwarder metrics

The log forwarder Function can emit its own custom metrics directly to New Relic's Metric API, in addition to forwarding logs. This is controlled via the `metrics_tier` Terraform variable (`none` / `basic` / `advanced`; default `basic`). `basic` covers core health (record counts, delivery success/loss, delivery latency, pipeline lag); `advanced` adds deeper root-cause/tuning metrics (byte volumes, decode/serialize errors, batching behavior, delivery error classes, run duration, secret-fetch failures, client-cache hit rate) on top of everything in `basic`.

## Versioning and releases

The function and the Resource Manager (ORM) template are versioned and released independently.

| Component | Version file | What is released | Release tag |
|---|---|---|---|
| Function (Go log forwarder) | [`logs-function/VERSION`](logs-function/VERSION) | Image pushed to OCIR in every region as `<version>` and `latest` | `function-<version>` |
| Template (ORM stack, `terraform/`) | [`terraform/VERSION`](terraform/VERSION) | `oci-log-integration.zip` | `<version>` |

* **Function:** any change under `logs-function/**` must bump `logs-function/VERSION`. A `<version>` image tag is never overwritten once published, and `latest` moves to the newest release.
* **Template:** any change under `terraform/**` that ships to customers must bump `terraform/VERSION` and `version:` in `terraform/schema.yaml`. The `Check Template Version` workflow fails the pull request if they differ.
* **Default and pinning:** `image_version` defaults to `latest`. To pin a specific function release, set `image_version` to that number (for example `1.3.0`). Pinning also lets you roll back by setting the previous number again.
* **Mapping:** the template does not reference a function version, so the two do not have to be released together. When a template change needs a newer function (for example a new variable the function must understand), record the minimum function version in the table below in the same pull request.

### Version compatibility

| Template (ORM) version | Minimum function version | Terraform module (`terraform-provider-newrelic`) | Notes |
|---|---|---|---|
| 1.2.0 | any (`latest`) | set at the next module release | Current template. |

Function releases: `1.3.0` is the first release with a versioned image. Earlier function builds were published as `latest` only, so they cannot be pinned.

## Contributing

We encourage your contributions to improve oci-log-integration! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project. If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company, please drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

## License

oci-log-integration is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
