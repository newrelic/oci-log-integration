data "oci_identity_tenancy" "current_tenancy" {
  tenancy_id = var.tenancy_ocid
}

# Human-readable name (not OCID) for the compartment this stack deploys into, so multiple
# forwarders reporting into one New Relic account can be told apart in dashboards.
#
# Skipped when deploying into the tenancy's root compartment: OCI's GetCompartment API
# can't resolve it (its OCID equals the tenancy OCID, but it isn't a fetchable Compartment
# resource -- only GetTenancy can return it), so calling this unconditionally would fail
# terraform apply for that (common, org-wide-logging) deployment choice. See local.compartment_name.
data "oci_identity_compartment" "current_compartment" {
  count = local.is_root_compartment ? 0 : 1
  id    = local.compartment_ocid
}

data "oci_identity_region_subscriptions" "subscriptions" {
  tenancy_id = var.tenancy_ocid
}

data "oci_core_subnet" "input_subnet" {
  depends_on = [module.vcn]
  subnet_id  = var.create_vcn ? module.vcn[0].subnet_id[local.subnet] : var.function_subnet_id
}

data "oci_resourcemanager_stacks" "current_stack" {
  compartment_id = var.compartment_ocid

  filter {
    name   = "display_name"
    values = [".*oci-log-integration.*"]
    regex  = true
  }
}

data "external" "connector_payload" {
  program = ["python", "${path.module}/connector.py"]
  query = {
    "payload_link" = var.payload_link
  }
}

data "oci_secrets_secretbundle" "user_api_key" {
  secret_id = local.user_key_secret_ocid
  provider  = oci.home_provider
}

# Resolved on every plan, so re-applying the stack notices when the tag points at a new image.
data "external" "function_image" {
  program = ["python", "${path.module}/image_mirror.py", "resolve"]
  query = {
    source_image = var.function_image
    platform     = "linux/amd64"
  }
}

data "oci_identity_user" "registry_user" {
  count    = var.registry_username == "" ? 1 : 0
  user_id  = var.current_user_ocid
  provider = oci.home_provider
}

# Only fetched when we're about to create a token (see oci_identity_auth_token.registry_push's
# precondition) -- OCI caps auth tokens at 2 per user, and creating a 3rd fails with a raw API
# error. Checking the existing count lets us fail with a clear message instead.
data "oci_identity_auth_tokens" "existing" {
  count    = local.create_registry_token ? 1 : 0
  user_id  = var.current_user_ocid
  provider = oci.home_provider
}