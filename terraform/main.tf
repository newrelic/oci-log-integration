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
  
  freeform_tags = {
    newrelic-logging-terraform = "true"
  }
  
  
  # Names for the network infra
  vcn_name        = "newrelic-logging-vcn"
  nat_gateway     = "${local.vcn_name}-natgateway"
  service_gateway = "${local.vcn_name}-servicegateway"
  subnet          = "${local.vcn_name}-public-subnet"
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
    "Allow dynamic-group ${var.dynamic_group_name} to use fn-invocation in tenancy"
  ]
  defined_tags  = {}
  freeform_tags = local.freeform_tags
}

#Resource for the function application
resource "oci_functions_application" "logging_function_app" {
  depends_on     = [oci_identity_policy.nr_logging_policy]
  compartment_id = var.compartment_ocid
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
  image         = "${var.region}.ocir.io/idfmbxeaoavl/hrai-container-repo/newrelic-log-forwarder:latest"
} 

#Resource for the service connector hub-1
resource "oci_sch_service_connector" "nr_service_connector" {
  depends_on     = [oci_functions_function.logging_function]
  compartment_id = var.compartment_ocid
  display_name   = var.connector_hub_name

  # Source Configuration with Logging
  source {
    kind = "logging"

    log_sources {
      compartment_id = var.compartment_ocid
      log_group_id   = var.log_group_id
      log_id         = var.log_id
    }
  }

  # Target Configuration with Functions
  target {
    #Required
    kind = "functions"

    #Optional
    batch_size_in_kbs = 100
    batch_time_in_sec = 60
    compartment_id    = var.compartment_ocid
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
  compartment_id           = var.compartment_ocid
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
  compartment_id = var.compartment_ocid
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