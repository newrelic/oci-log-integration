terraform {
  required_version = ">= 1.2.0"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "5.46.0"
    }
  }
}

# Variables
provider "oci" {
  alias        = "home"
  tenancy_ocid = var.tenancy_ocid
  user_ocid    = data.oci_identity_user.current_user.user_id
  region       = var.region
}

data "oci_identity_user" "current_user" {
  user_id = var.current_user_ocid
}

data "oci_identity_region_subscriptions" "subscriptions" {
  tenancy_id = var.tenancy_ocid
}

data "oci_identity_tenancy" "current_tenancy" {
  tenancy_id = var.tenancy_ocid
}



locals {

  compartment_ocid_for_core_resources = var.tenancy_ocid

  freeform_tags = {
    newrelic-logging-terraform = "true"
  }

  vcn_name        = "newrelic-logging-vcn"
  nat_gateway     = "${local.vcn_name}-natgateway"
  service_gateway = "${local.vcn_name}-servicegateway"
  subnet          = "${local.vcn_name}-public-subnet"
}

# Data source to find the compartment OCID for EACH log source.
# This uses for_each to iterate over the provided log_sources.
data "oci_identity_compartments" "log_source_compartments" {
  for_each = {
    for idx, source in var.log_sources : idx => source
    # Only run this data source if the compartment name is NOT the tenancy name
    if source.compartment_name != data.oci_identity_tenancy.current_tenancy.name
  }

  # Search within the root (tenancy) for the named child compartment
  compartment_id = var.tenancy_ocid

  filter {
    name  = "name"
    values = [each.value.compartment_name]
  }
}

# Lookup log group OCID by name for EACH log source
data "oci_logging_log_groups" "log_source_groups" {
  for_each = { for idx, source in var.log_sources : idx => source }

  # Dynamically determine the compartment_id for the log group:
  # If the compartment_name matches the tenancy's display name, use tenancy_ocid directly.
  # Otherwise, use the OCID found by the log_source_compartments data source for that key.
  # Ensure the entire ternary expression is on a single logical line or properly continued
  compartment_id = each.value.compartment_name == data.oci_identity_tenancy.current_tenancy.name ? var.tenancy_ocid : data.oci_identity_compartments.log_source_compartments[each.key].compartments[0].id

  filter {
    name  = "display_name"
    values = [each.value.log_group_name]
  }
}

# Lookup log OCID by name for EACH log source (if log_name is provided)
data "oci_logging_logs" "log_source_logs" {
  for_each = { for idx, source in var.log_sources : idx => source if source.log_name != "" } # Only lookup if log_name is not empty

  # Use the dynamically found log group OCID for this specific log source
  log_group_id = data.oci_logging_log_groups.log_source_groups[each.key].log_groups[0].id
  filter {
    name  = "display_name"
    values = [each.value.log_name]
  }
}












#Resource for the dynamic group
resource "oci_identity_dynamic_group" "nr_serviceconnector_group_hrai" {
  compartment_id = var.tenancy_ocid
  description    = "[DO NOT REMOVE] Dynamic group for service connector"
  matching_rule  = "All {resource.type = 'serviceconnector'}"
  name           = var.dynamic_group_name
  defined_tags   = {}
  freeform_tags  = local.freeform_tags
}

# Resource for the policy 
resource "oci_identity_policy" "nr_logging_policy" {
  depends_on     = [oci_identity_dynamic_group.nr_serviceconnector_group_hrai]
  compartment_id = var.tenancy_ocid
  description    = "[DO NOT REMOVE] Policy to have any connector hub read from logging source and write to a target function"
  name           = var.newrelic_logging_policy
  statements     = [
    "Allow dynamic-group ${var.dynamic_group_name} to read logs in tenancy",
    "Allow dynamic-group ${var.dynamic_group_name} to use fn-function in tenancy",
    "Allow dynamic-group ${var.dynamic_group_name} to use fn-invocation in tenancy",
    "Allow dynamic-group ${var.dynamic_group_name} to inspect log-groups in tenancy"
  ]
  defined_tags  = {}
  freeform_tags = local.freeform_tags
}

#Resource for the function application
resource "oci_functions_application" "logging_function_app" {
  depends_on     = [oci_identity_policy.nr_logging_policy]
  compartment_id = local.compartment_ocid_for_core_resources
  config = {
    "NEW_RELIC_LICENSE_KEY"  = var.newrelic_api_key
  }
  defined_tags               = {}
  display_name               = var.newrelic_function_app
  freeform_tags              = local.freeform_tags
  network_security_group_ids = []
  shape                      = var.function_app_shape
  subnet_ids = [
    module.vcn[0].subnet_id[local.subnet], 
  ]
}

#Resource for the function
resource "oci_functions_function" "logging_function" {
  depends_on = [oci_functions_application.logging_function_app]

  application_id = oci_functions_application.logging_function_app.id
  display_name   = "${oci_functions_application.logging_function_app.display_name}-logging-function"
  memory_in_mbs  = "256"

  defined_tags  = {}
  freeform_tags = local.freeform_tags
  image         = "${var.region}.ocir.io/idms1yfytybe/oci-testing-registry/oci-function-x86:0.0.1"
} 

# hrai repo details 
#image         = "${var.region}.ocir.io/idfmbxeaoavl/hrai-container-repo/newrelic-log-forwarder:latest"

# idms1yfytybe  -> beyond-nr-1-account
# oci-testing-registry/oci-function-x86:0.0.1



module "vcn" {
  source                   = "oracle-terraform-modules/vcn/oci"
  version                  = "3.6.0"
  count                    = 1
  compartment_id           = local.compartment_ocid_for_core_resources
  defined_tags             = {}
  freeform_tags            = local.freeform_tags
  vcn_cidrs                = ["10.0.0.0/16"]
  vcn_dns_label            = "nrlogging"
  vcn_name                 = local.vcn_name
  lockdown_default_seclist = false
  subnets = {
    public = {
      cidr_block = "10.0.0.0/16"
      type       = "public"
      name       = local.subnet
    }
  }
  create_nat_gateway           = true
  nat_gateway_display_name     = local.nat_gateway
  create_service_gateway       = true
  service_gateway_display_name = local.service_gateway
  create_internet_gateway      = true # Enable creation of Internet Gateway
  internet_gateway_display_name = "NRLoggingInternetGateway" # Name the Internet Gateway
}

data "oci_core_route_tables" "default_vcn_route_table" {
  depends_on     = [module.vcn] # Ensure VCN is created before attempting to find its route tables
  compartment_id = local.compartment_ocid_for_core_resources
  vcn_id         = module.vcn[0].vcn_id # Get the VCN ID from the module output

  filter {
    name   = "display_name"
    values = ["Default Route Table for ${local.vcn_name}"]
    regex  = false
  }
}

# Resource to manage the VCN's default route table and add your rule.
resource "oci_core_default_route_table" "default_internet_route" {
  manage_default_resource_id = data.oci_core_route_tables.default_vcn_route_table.route_tables[0].id
  depends_on = [
    module.vcn,
    data.oci_core_route_tables.default_vcn_route_table # Ensure the data source has run
  ]
  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = module.vcn[0].internet_gateway_id # Reference the internet gateway created by the module
    description       = "Route to Internet Gateway for New Relic logging"
  }

}

# --- Service Connector Hub for multiple log sources ---
resource "oci_sch_service_connector" "nr_service_connector" {
  depends_on     = [oci_functions_function.logging_function]
  compartment_id = local.compartment_ocid_for_core_resources
  display_name   = var.connector_hub_name

  # Source Configuration with Logging
  source {
    kind = "logging"

    # Create a log_sources block for each log source
    dynamic "log_sources" {
      for_each = var.log_sources
      content {
        # Use the correct compartment resolution logic
        compartment_id = log_sources.value.compartment_name == data.oci_identity_tenancy.current_tenancy.name ? var.tenancy_ocid : data.oci_identity_compartments.log_source_compartments[log_sources.key].compartments[0].id
        
        # Use the correct data source names
        log_group_id = data.oci_logging_log_groups.log_source_groups[log_sources.key].log_groups[0].id
        
        # Conditionally set log_id if log_name is provided
        log_id = log_sources.value.log_name != "" ? data.oci_logging_logs.log_source_logs[log_sources.key].logs[0].id : null
      }
    }
  }

  # Target Configuration with Functions
  target {
    kind              = "functions"
    batch_size_in_kbs = 100
    batch_time_in_sec = 60
    compartment_id    = local.compartment_ocid_for_core_resources
    function_id       = oci_functions_function.logging_function.id
  }

  description   = "Service Connector for multiple OCI log sources to New Relic Functions"
  defined_tags  = {}
  freeform_tags = local.freeform_tags
}