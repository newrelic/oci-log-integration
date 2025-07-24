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

# Data source to find the target compartment by name.
# It explicitly excludes the root compartment from its search by not setting
# compartment_id to var.tenancy_ocid. Instead, it queries the sub-compartments.
# We will use 'current_tenancy.name' to detect if 'compartment_name' refers to the root.

data "oci_identity_compartments" "target_child_compartment" {

  # Only try to find a child compartment if var.compartment_name is NOT the tenancy name.
  count = var.compartment_name != data.oci_identity_tenancy.current_tenancy.name ? 1 : 0
  
  # Search within the root (tenancy) for the named child compartment
  compartment_id = var.tenancy_ocid 
  filter {
    name   = "name"
    values = [var.compartment_name]
  }
}


locals {
  # This crucial local determines the compartment_ocid for resource deployment.
  # If var.compartment_name matches the tenancy's display name, use tenancy_ocid (root).
  # Otherwise, attempt to find a child compartment by name.
  # If no child compartment is found (length is 0), it implies an error or misconfiguration,
  # but for robustness, we'll fall back to tenancy_ocid.
  
  compartment_ocid = var.compartment_name == data.oci_identity_tenancy.current_tenancy.name ? var.tenancy_ocid : (
    length(data.oci_identity_compartments.target_child_compartment) > 0 ? data.oci_identity_compartments.target_child_compartment[0].id : var.tenancy_ocid
  )

  log_group_compartment_ocid_for_lookup = local.compartment_ocid
  # For log sources, if they might be in a different compartment than the deployed resources,
  # you'd need a separate variable (e.g., var.log_group_compartment_ocid)
  # For now, assuming log group is in the same compartment as other resources.
  freeform_tags = {
    newrelic-logging-terraform = "true"
  }

  vcn_name        = "newrelic-logging-vcn"
  nat_gateway     = "${local.vcn_name}-natgateway"
  service_gateway = "${local.vcn_name}-servicegateway"
  subnet          = "${local.vcn_name}-public-subnet"
}

# Lookup log group OCID by name
data "oci_logging_log_groups" "target" {
  compartment_id = local.log_group_compartment_ocid_for_lookup  # Use the determined compartment
  filter {
    name   = "display_name"
    values = [var.log_group_name]
  }
}






locals {
  log_group_ocid = data.oci_logging_log_groups.target.log_groups[0].id
}

# Lookup log OCID by name
data "oci_logging_logs" "target" {
  # Only attempt lookup if var.log_name is not empty
  count = var.log_name != "" ? 1 : 0
  log_group_id = local.log_group_ocid
  filter {
    name   = "display_name"
    values = [var.log_name]
  }
}


locals {
  # If log_name was provided, get the log OCID; otherwise, it's null (meaning all logs in group)
  log_ocid = var.log_name != "" ? data.oci_logging_logs.target[0].logs[0].id : null
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
  compartment_id = local.compartment_ocid
  config = {
    "NEW_RELIC_LICENSE_KEY"  = var.newrelic_api_key
  }
  defined_tags               = {}
  display_name               = var.newrelic_function_app
  freeform_tags              = local.freeform_tags
  network_security_group_ids = []
  shape                      = var.function_app_shape
  subnet_ids = [
    module.vcn[0].subnet_id[local.subnet], # Corrected reference
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

#Resource for the service connector hub-1
resource "oci_sch_service_connector" "nr_service_connector" {
  depends_on     = [oci_functions_function.logging_function]
  compartment_id = local.compartment_ocid
  display_name   = var.connector_hub_name

  # Source Configuration with Logging
  source {
    kind = "logging"

    log_sources {
      compartment_id = local.log_group_compartment_ocid_for_lookup
      log_group_id   = local.log_group_ocid
      log_id         = local.log_ocid
    }
  }

  # Target Configuration with Functions
  target {
    #Required
    kind = "functions"

    #Optional
    batch_size_in_kbs = 100
    batch_time_in_sec = 60
    compartment_id    = local.compartment_ocid
    function_id       = oci_functions_function.logging_function.id
  }

  # Optional tags and additional metadata
  description   = "Service Connector from Logging to Functions"
  defined_tags  = {}
  freeform_tags = {}
}


module "vcn" {
  source                   = "oracle-terraform-modules/vcn/oci"
  version                  = "3.6.0"
  count                    = 1
  compartment_id           = local.compartment_ocid
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
  compartment_id = local.compartment_ocid
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