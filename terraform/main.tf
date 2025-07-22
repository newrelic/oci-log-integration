# OCI Log Integration with New Relic - Terraform Template
# This template creates all necessary resources to forward OCI logs to New Relic

terraform {
  required_version = ">= 1.0"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 5.0"
    }
  }
}

# Get current tenancy information
data "oci_identity_tenancy" "current_tenancy" {
  tenancy_id = var.tenancy_ocid
}

# Get availability domains
data "oci_identity_availability_domains" "ads" {
  compartment_id = var.tenancy_ocid
}

# Virtual Cloud Network (VCN)
resource "oci_core_vcn" "newrelic_log_vcn" {
  count          = var.create_new_vcn ? 1 : 0
  compartment_id = var.compartment_ocid
  cidr_block     = var.vcn_cidr_block
  display_name   = "${var.resource_prefix}-vcn"
  dns_label      = "nrlogvcn"
  
  freeform_tags = var.freeform_tags
}

# Internet Gateway
resource "oci_core_internet_gateway" "newrelic_log_igw" {
  count          = var.create_new_vcn ? 1 : 0
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.newrelic_log_vcn[0].id
  display_name   = "${var.resource_prefix}-igw"
  enabled        = true
  
  freeform_tags = var.freeform_tags
}

# Route Table
resource "oci_core_route_table" "newrelic_log_rt" {
  count          = var.create_new_vcn ? 1 : 0
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.newrelic_log_vcn[0].id
  display_name   = "${var.resource_prefix}-rt"
  
  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = oci_core_internet_gateway.newrelic_log_igw[0].id
  }
  
  freeform_tags = var.freeform_tags
}

# Security List
resource "oci_core_security_list" "newrelic_log_sl" {
  count          = var.create_new_vcn ? 1 : 0
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.newrelic_log_vcn[0].id
  display_name   = "${var.resource_prefix}-sl"

  egress_security_rules {
    destination      = "0.0.0.0/0"
    destination_type = "CIDR_BLOCK"
    protocol         = "all"
    description      = "Allow all outbound traffic"
  }

  ingress_security_rules {
    protocol    = "6" # TCP
    source      = "0.0.0.0/0"
    source_type = "CIDR_BLOCK"
    description = "Allow HTTPS inbound"
    
    tcp_options {
      max = 443
      min = 443
    }
  }
  
  freeform_tags = var.freeform_tags
}

# Public Subnet
resource "oci_core_subnet" "newrelic_log_subnet" {
  count                      = var.create_new_vcn ? 1 : 0
  compartment_id             = var.compartment_ocid
  vcn_id                     = oci_core_vcn.newrelic_log_vcn[0].id
  cidr_block                 = var.subnet_cidr_block
  display_name               = "${var.resource_prefix}-subnet"
  dns_label                  = "nrlogsubnet"
  route_table_id             = oci_core_route_table.newrelic_log_rt[0].id
  security_list_ids          = [oci_core_security_list.newrelic_log_sl[0].id]
  prohibit_public_ip_on_vnic = false
  
  freeform_tags = var.freeform_tags
}

# Dynamic Group for Function
resource "oci_identity_dynamic_group" "newrelic_log_forwarder_dg" {
  compartment_id = var.tenancy_ocid
  name           = "${var.resource_prefix}-function-dg"
  description    = "Dynamic group for New Relic log forwarder function"
  matching_rule  = "ALL {resource.type = 'fnfunc', resource.compartment.id = '${var.compartment_ocid}'}"
  
  freeform_tags = var.freeform_tags
}

# IAM Policy for Service Connector Hub
resource "oci_identity_policy" "service_connector_policy" {
  compartment_id = var.tenancy_ocid
  name           = "${var.resource_prefix}-sch-policy"
  description    = "Policy for Service Connector Hub to read logs and invoke functions"
  
  statements = [
    "Allow service service-connector-hub to read log-groups in compartment id ${var.compartment_ocid}",
    "Allow service service-connector-hub to read log-content in compartment id ${var.compartment_ocid}",
    "Allow service service-connector-hub to use functions-family in compartment id ${var.compartment_ocid}",
    "Allow service service-connector-hub to manage log-groups in compartment id ${var.compartment_ocid}",
    "Allow service service-connector-hub to manage log-content in compartment id ${var.compartment_ocid}"
  ]
  
  freeform_tags = var.freeform_tags
}

# IAM Policy for Function
resource "oci_identity_policy" "function_policy" {
  compartment_id = var.compartment_ocid
  name           = "${var.resource_prefix}-function-policy"
  description    = "Policy for New Relic log forwarder function"
  
  statements = [
    "Allow dynamic-group ${oci_identity_dynamic_group.newrelic_log_forwarder_dg.name} to use log-content in compartment id ${var.compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.newrelic_log_forwarder_dg.name} to read repos in compartment id ${var.compartment_ocid}"
  ]
  
  freeform_tags = var.freeform_tags
}

# OCI Functions Application
resource "oci_functions_application" "newrelic_log_app" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.resource_prefix}-app"
  subnet_ids     = var.create_new_vcn ? [oci_core_subnet.newrelic_log_subnet[0].id] : [var.existing_subnet_ocid]
  
  freeform_tags = var.freeform_tags
}

# OCI Function for log forwarding
resource "oci_functions_function" "newrelic_log_forwarder" {
  application_id     = oci_functions_application.newrelic_log_app.id
  display_name       = "${var.resource_prefix}-function"
  image              = var.function_docker_image
  memory_in_mbs      = var.function_memory_mb
  timeout_in_seconds = var.function_timeout_seconds
  
  config = {
    "NEWRELIC_LOGS_ENDPOINT" = var.newrelic_logs_endpoint
    "NEWRELIC_INGEST_KEY"    = var.newrelic_ingest_key
  }

  depends_on = [
    oci_identity_policy.function_policy,
    oci_identity_dynamic_group.newrelic_log_forwarder_dg
  ]
  
  freeform_tags = var.freeform_tags
}

# Service Connector Hub
resource "oci_sch_service_connector" "newrelic_log_connector" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.resource_prefix}-connector"
  description    = "Forwards logs from OCI Log Group to New Relic via function"
  state          = "ACTIVE"
  
  source {
    kind = "logging"
    
    log_sources {
      compartment_id = var.compartment_ocid
      log_group_id   = var.log_group_ocid
      log_id = var.log_ocid
    }
  }
  
  target {
    kind        = "functions"
    function_id = oci_functions_function.newrelic_log_forwarder.id
  }
  
  freeform_tags = var.freeform_tags
  
  depends_on = [
    oci_identity_policy.service_connector_policy,
    oci_functions_function.newrelic_log_forwarder
  ]
}

# Outputs
output "vcn_id" {
  description = "OCID of the VCN (if created)"
  value       = var.create_new_vcn ? oci_core_vcn.newrelic_log_vcn[0].id : "Using existing VCN"
}

output "subnet_id" {
  description = "OCID of the subnet used by the function"
  value       = var.create_new_vcn ? oci_core_subnet.newrelic_log_subnet[0].id : var.existing_subnet_ocid
}

output "function_application_id" {
  description = "OCID of the Functions Application"
  value       = oci_functions_application.newrelic_log_app.id
}

output "function_id" {
  description = "OCID of the New Relic Log Forwarder Function"
  value       = oci_functions_function.newrelic_log_forwarder.id
}

output "function_invoke_endpoint" {
  description = "Invoke endpoint URL for the function"
  value       = oci_functions_function.newrelic_log_forwarder.invoke_endpoint
}

output "service_connector_id" {
  description = "OCID of the Service Connector Hub"
  value       = oci_sch_service_connector.newrelic_log_connector.id
}

output "dynamic_group_id" {
  description = "OCID of the Dynamic Group created for the function"
  value       = oci_identity_dynamic_group.newrelic_log_forwarder_dg.id
}

output "setup_complete" {
  description = "Confirmation that the setup is complete"
  value       = "New Relic log integration setup completed successfully. Logs from Log Group ${var.log_group_ocid} will be forwarded to New Relic."
}
