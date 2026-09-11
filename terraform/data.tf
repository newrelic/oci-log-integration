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