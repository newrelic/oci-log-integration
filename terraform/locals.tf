
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
  function_app_name           = "newrelic-${var.newrelic_logging_identifier}-${var.region}-logs-function-app"
  function_app_shape          = "GENERIC_X86"
  client_ttl                  = 30
  function_app_log_group_name = "newrelic-${var.newrelic_logging_identifier}-${var.region}-logs-function-log-group"
  function_app_log_name       = "newrelic-${var.newrelic_logging_identifier}-${var.region}-logs-function-log"

  # Function Constants
  function_name       = "newrelic-${var.newrelic_logging_identifier}-${var.region}-logs-function"
  memory_in_mbs       = "128"
  time_out_in_seconds = 300

  # The function runs from a copy of var.function_image in the tenancy's own Container
  # Registry, not from New Relic's own OCIR (the old image_url path this replaces).
  ocir_host = "${var.region}.ocir.io"
  ocir_namespace = local.create_container_repository ? (
    oci_artifacts_container_repository.log_forwarder_repo[0].namespace
    ) : (
    data.oci_artifacts_container_repository.existing[0].namespace
  )
  function_image_repository = "newrelic-${var.newrelic_logging_identifier}/oci-log-forwarder"
  function_image_digest     = data.external.function_image.result.digest
  function_image            = "${local.ocir_host}/${local.ocir_namespace}/${local.function_image_repository}:${data.external.function_image.result.tag}"

  # nonsensitive() here is safe: this only reveals whether the token string was empty, never
  # the token itself. It's required because count/for_each reject sensitive values outright.
  create_container_repository     = nonsensitive(var.existing_container_repository_id == "")
  create_registry_token           = nonsensitive(var.registry_auth_token == "")
  registry_push_token_description = "New Relic logs stack: pushes the function image to ${local.function_image_repository} in ${var.region}"
  # data.oci_identity_auth_tokens.existing is re-read on every plan, so once this stack has
  # created its own token, that token itself shows up in the list -- excluding it here is what
  # keeps the 2-token-cap precondition in main.tf from failing on every subsequent re-apply.
  other_existing_auth_tokens = local.create_registry_token ? [
    for t in data.oci_identity_auth_tokens.existing[0].tokens : t
    if t.description != local.registry_push_token_description
  ] : []
  registry_username = "${local.ocir_namespace}/${var.registry_username != "" ? var.registry_username : data.oci_identity_user.registry_user[0].name}"
  # Left sensitive (no nonsensitive() here) so a customer-supplied token stays masked in
  # plan/apply console output and whatever captures it (ORM/CI job logs). Terraform suppresses
  # the whole local-exec invocation's output when this branch is taken, including image_mirror.py's
  # progress log -- that's the correct trade: no partial masking exists, so visibility loses to
  # not leaking the credential. When we create the token ourselves instead (the other branch),
  # progress stays visible, because terraform-provider-oci's oci_identity_auth_token resource never
  # marks its own `token` attribute Sensitive -- a provider-level gap outside this module's control,
  # so that value is already in the Terraform state in plaintext regardless of what we do here.
  registry_password = local.create_registry_token ? oci_identity_auth_token.registry_push[0].token : var.registry_auth_token

  # connector hub config
  batch_size_in_kbs = 6000
  batch_time_in_sec = 60

  connectors             = jsondecode(data.external.connector_payload.result.connectors)
  compartment_ocid       = data.external.connector_payload.result.compartment_id
  ingest_key_secret_ocid = data.external.connector_payload.result.ingest_key_secret_ocid
  user_key_secret_ocid   = data.external.connector_payload.result.user_key_secret_ocid
  providerAccountId      = data.external.connector_payload.result.provider_account_id

  # The tenancy's root compartment shares its OCID with the tenancy itself, but OCI's
  # Identity service can't return it from GetCompartment (data.oci_identity_compartment) --
  # only GetTenancy can. Detect that case so we read the name from the right data source.
  is_root_compartment = local.compartment_ocid == var.tenancy_ocid
  compartment_name    = local.is_root_compartment ? data.oci_identity_tenancy.current_tenancy.name : data.oci_identity_compartment.current_compartment[0].name

  user_api_key = base64decode(data.oci_secrets_secretbundle.user_api_key.secret_bundle_content[0].content)
  stack_id     = data.oci_resourcemanager_stacks.current_stack.stacks[0].id

  connectors_map = {
    for conn in local.connectors : conn.display_name => conn
  }
  newrelic_graphql_endpoint = {
    US = "https://api.newrelic.com/graphql"
    EU = "https://api.eu.newrelic.com/graphql"
    JP = "https://api.jp.newrelic.com/graphql"
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