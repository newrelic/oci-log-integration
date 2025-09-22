
locals {

  home_region = [
    for rs in data.oci_identity_region_subscriptions.subscriptions.region_subscriptions :
    rs.region_name if rs.region_key == data.oci_identity_tenancy.current_tenancy.home_region_key
  ][0]

  freeform_tags = {
    newrelic-terraform = "true"
  }
  # Names for the network infra
  vcn_name        = "newrelic-${var.nr_prefix}-${var.region}-logs-vcn"
  nat_gateway     = "${local.vcn_name}-natgateway"
  service_gateway = "${local.vcn_name}-servicegateway"
  subnet          = "${local.vcn_name}-private-subnet"

  connectors       = jsondecode(data.external.connector_payload.result.connectors)
  compartment_ocid = data.external.connector_payload.result.compartment_id
  ingest_key_secret_ocid = data.external.connector_payload.result.ingest_key_secret_ocid
  user_key_secret_ocid = data.external.connector_payload.result.user_key_secret_ocid
  providerAccountId     = data.external.connector_payload.result.provider_account_id

  user_api_key          = base64decode(data.oci_secrets_secretbundle.user_api_key.secret_bundle_content[0].content)
  stack_id              = data.oci_resourcemanager_stacks.current_stack.stacks[0].id
  
  connectors_map = {
    for conn in local.connectors : conn.display_name => conn
  }
  newrelic_graphql_endpoint = "https://api.newrelic.com/graphql"
  updateLinkAccount_graphql_query = <<EOF
mutation {
  cloudUpdateAccount(
    accountId: ${var.newrelic_account_id}
    accounts = {
      oci = {
        compartmentOcid: "${local.compartment_ocid}"
        linkedAccountId: "${local.providerAccountId}"
        loggingStackOcid: "${local.stack_id}"
        ociHomeRegion: "${local.home_region}"
        tenantId: "${var.tenancy_ocid}"
        ociRegion: "${var.region}"
        userVaultOcid: "${local.user_key_secret_ocid}"
        ingestVaultOcid: "${local.ingest_key_secret_ocid}"
      }
  }
) {
    errors {
      linkedAccountId
      providerSlug
      message
      nrAccountId
      type
    }
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