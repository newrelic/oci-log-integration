
locals {

  home_region = [
    for rs in data.oci_identity_region_subscriptions.subscriptions.region_subscriptions :
    rs.region_name if rs.region_key == data.oci_identity_tenancy.current_tenancy.home_region_key
  ][0]

  freeform_tags = {
    newrelic-terraform = "true"
  }

  # Names for the network infra
  vcn_name         = "newrelic-${var.newrelic_logging_identifier}-${var.region}-logs-vcn"
  nat_gateway      = "newrelic-${var.newrelic_logging_identifier}-${var.region}-natgateway"
  service_gateway  = "newrelic-${var.newrelic_logging_identifier}-${var.region}-servicegateway"
  subnet           = "newrelic-${var.newrelic_logging_identifier}-${var.region}-private-subnet"
  internet_gateway = "newrelic-${var.newrelic_logging_identifier}-${var.region}-internetgateway"

  # Function App Constants
  function_app_name  = "newrelic-${var.newrelic_logging_identifier}-${var.region}-logs-function-app"
  function_app_shape = "GENERIC_X86"
  client_ttl         = 30
  function_app_log_group_name = "${var.newrelic_logging_identifier}-${var.region}-function-app-log-group"
  function_app_log_name       = "${local.function_app_name}-execution-log"

  # Function Constants
  function_name          = "newrelic-${var.newrelic_logging_identifier}-${var.region}-logs-function"
  memory_in_mbs = "128"
  time_out_in_seconds    = 300
  image_url              = "${var.region}.ocir.io/idptojlonu4e/newrelic-logs-integration/oci-log-forwarder:${var.image_version}"

  # connector hub config
  batch_size_in_kbs = 6000
  batch_time_in_sec = 60

  connectors             = jsondecode(data.external.connector_payload.result.connectors)
  compartment_ocid       = data.external.connector_payload.result.compartment_id
  ingest_key_secret_ocid = data.external.connector_payload.result.ingest_key_secret_ocid
  user_key_secret_ocid   = data.external.connector_payload.result.user_key_secret_ocid
  providerAccountId      = data.external.connector_payload.result.provider_account_id

  user_api_key = base64decode(data.oci_secrets_secretbundle.user_api_key.secret_bundle_content[0].content)
  stack_id     = data.oci_resourcemanager_stacks.current_stack.stacks[0].id

  connectors_map = {
    for conn in local.connectors : conn.display_name => conn
  }
  newrelic_graphql_endpoint = {
    US = "https://api.newrelic.com/graphql"
    EU = "https://api.eu.newrelic.com/graphql"
  }[var.new_relic_region]
  updateLinkAccount_graphql_query = <<EOF
mutation {
  cloudUpdateAccount(
    accountId: ${var.newrelic_account_id}
    accounts: {
      oci: {
        compartmentOcid: "${local.compartment_ocid}"
        linkedAccountId: ${local.providerAccountId}
        loggingStackOcid: "${local.stack_id}"
        ociRegion: "${var.region}"
        userVaultOcid: "${local.user_key_secret_ocid}"
        ingestVaultOcid: "${local.ingest_key_secret_ocid}"
      }
  }
) {
    linkedAccounts {
      id
      authLabel
      createdAt
      disabled
      externalId
      metricCollectionMode
      name
      nrAccountId
      updatedAt
    }
  }
}
EOF
}