
# OCI Tenancy and User Variables
variable "tenancy_ocid" {
  type        = string
  description = "OCI tenant OCID, more details can be found at https://docs.cloud.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five"
}

variable "current_user_ocid" {
  type        = string
  description = "The OCID of the current user executing the terraform script. Do not modify."
}

variable "compartment_ocid" {
  type        = string
  description = "The OCID of the compartment where resources will be created."
}

# OCI Logging Integration Resource Variables
variable "function_app_name" {
  type        = string
  description = "The name of the function application"
  default     = "newrelic-logs-function-app"
}

variable "function_app_shape" {
  type        = string
  default     = "GENERIC_X86"
  description = "The shape of the function application. The docker image should be built accordingly. Use ARM if using Oracle Resource manager stack"
}

variable "connector_hub_name" {
  type        = string
  description = "The prefix for the name of all of the resources"
  default     = "newrelic-logs-connector-hub"
}

variable "newrelic_logs_policy" {
  type        = string
  description = "Logging Integration Policy"
  default     = "newrelic-logs-policy"
}

variable "dynamic_group_name" {
  type        = string
  description = "The name of the dynamic group for giving access to service connector"
  default     = "newrelic-logging-dynamic-group"
}

variable "region" {
  type        = string
  description = "OCI Region as documented at https://docs.cloud.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm"
}

# Log Forwarding Function Variables
variable "tenancy-namespace" {
  type        = string
  description = "The tenancy namespace where function image resides"
}

variable "repository-name" {
  type        = string
  description = "The name of the repository for function image"
  default     = "newrelic-logging-repo"
}

variable "repository-version" {
    type        = string
    description = "The version of the repository for function image"
    default     = "latest"
}

# Log Forwarding Function Custom Variables
variable "nr_region" {
  type        = string
  description = "New Relic Region to forward Logs to. Valid values are 'US' or 'EU'."
  default = "US"
}

variable "license_key" {
  type        = string
  sensitive   = true
  description = "License Key to ingest logs"
}

variable "newrelic_account_id" {
  type        = string
  sensitive   = true
  description = "The New Relic account ID for sending logs to New Relic endpoints"
}

variable "log_group_id" {
  type        = string
  description = "log group OCID to send logs to New Relic."
}

variable "log_id" {
  type        = string
  description = "log OCID to send logs to New Relic."
}

variable "debug_enabled" {
  type        = string
  description = "Enable debug mode."
  default     = "false"
}