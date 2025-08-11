locals {
  source_tenancy_ocid = "ocid1.tenancy.oc1..your_source_tenancy_ocid"
  source_group_ocid   = "ocid1.group.oc1..your_source_group_ocid"
}

# ---------------------------------------------------------------------------------------------------------------------
# Customer Onboarding (Secret Vault, Dynamic Groups and Policies Creation)
# ---------------------------------------------------------------------------------------------------------------------
# These resources are deployed regardless of which (metrics, logs) integration the customer chooses.

# Cross-Tenancy New Relic Read-Only Access Policy
resource "oci_identity_policy" "cross_tenancy_read_only_policy" {
  compartment_id = var.compartment_ocid
  name           = "Cross_Tenancy_Read_Only_Policy"
  description    = "Policy granting New Relic tenancy read-only access to connector hubs, VCNs, and log groups."
  statements     = [
    "Define tenancy NRTenancyAlias as ${local.source_tenancy_ocid}",
    "Define group NRCustomerOCIAccessGroupAlias as ${local.source_group_ocid}",

    "Admit group NRCustomerOCIAccessGroupAlias of tenancy NRTenancyAlias to read all-resources in tenancy",
    "Admit group NRCustomerOCIAccessGroupAlias of tenancy NRTenancyAlias to read virtual-network-family in tenancy",
    "Admit group NRCustomerOCIAccessGroupAlias of tenancy NRTenancyAlias to read log-content in tenancy",
    "Admit group NRCustomerOCIAccessGroupAlias of tenancy NRTenancyAlias to read service-connector-hubs in tenancy",
  ]
}

# Policies for Connector Hubs in given Compartment
resource "oci_identity_dynamic_group" "connector_hub_dg" {
  compartment_id = var.tenancy_ocid
  name           = "Service_Connector_Hubs_DG"
  description    = "Dynamic group for all Service Connector Hubs in the specified compartment."
  matching_rule  = "ALL {resource.type = 'serviceconnector', instance.compartment.id = '${var.compartment_ocid}'}"
}

resource "oci_identity_policy" "connector_hub_policy" {
  compartment_id = var.compartment_ocid
  name           = "Connector_Hub_Log_Access"
  description    = "Allows connector hubs to read logs and trigger functions."
  statements     = [
    "Allow dynamic-group ${oci_identity_dynamic_group.connector_hub_dg.name} to read log-content in tenancy",
    "Allow dynamic-group ${oci_identity_dynamic_group.connector_hub_dg.name} to use fn-function in compartment id ${var.compartment_ocid}",
  ]
}

# Cross-Regional Vault Access for Functions
resource "oci_identity_dynamic_group" "all_functions_dg" {
  compartment_id = var.tenancy_ocid
  name           = "All_Functions_DG"
  description    = "Dynamic group for all functions in the specified compartment."
  matching_rule  = "ALL {instance.compartment.id = '${var.compartment_ocid}'}"
}

resource "oci_identity_policy" "functions_vault_access_policy" {
  compartment_id = var.compartment_ocid
  name           = "Functions_Vault_Access_Policy"
  description    = "Policy allowing functions to read secrets from the vault."
  statements     = [
    "Allow dynamic-group ${oci_identity_dynamic_group.all_functions_dg.name} to read secret-bundles in compartment id ${var.compartment_ocid}",
  ]
}


# ---------------------------------------------------------------------------------------------------------------------
# Conditional Metrics Module
# ---------------------------------------------------------------------------------------------------------------------
# This module is sourced from the metrics team's S3 bucket.
# The 'count' meta-argument is used to conditionally deploy the module.

# module "new_relic_metrics" {
#   count  = var.deploy_metrics ? 1 : 0
  # source = local.metrics_template_s3_url

  # Pass variables from the wrapper to the metrics module.
  # The module's variables.tf must accept these inputs.
  # tenancy_ocid         = var.tenancy_ocid
  # compartment_ocid     = var.compartment_ocid
# }

# ---------------------------------------------------------------------------------------------------------------------
# Conditional Logs Module
# ---------------------------------------------------------------------------------------------------------------------
# This module is sourced from the logs team's S3 bucket.

# module "new_relic_logs" {
#   count  = var.deploy_logs ? 1 : 0
  # source = var.logs_template_s3_url

  # Pass variables from the wrapper to the logs module.
  # The module's variables.tf must accept these inputs.
  # tenancy_ocid         = var.tenancy_ocid
  # compartment_ocid     = var.compartment_ocid
# }