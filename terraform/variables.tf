variable "tenancy_ocid" {
  type        = string
  description = "OCI tenant OCID, more details can be found at https://docs.cloud.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five"
}

variable "current_user_ocid" {
  type        = string
  description = "The OCID of the current user executing the terraform script. Do not modify."
}

variable "compartment_name" {
  type        = string
  description = "The name of the compartment where resources will be created."
}

variable "region" {
  type        = string
  description = "OCI Region as documented at https://docs.cloud.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm"
}

variable "dynamic_group_name" {
  type        = string
  description = "The name of the dynamic group for giving access to service connector"
  default     = "newrelic-logging-dynamic-group-hrai"
}

variable "newrelic_logging_policy" {
  type        = string
  description = "The name of the policy for logging"
  default     = "newrelic-logging-policy"
}


variable "newrelic_logging_endpoint" {
  type        = string
  default     = "https://log-api.newrelic.com/log/v1"
  description = "The endpoint to hit for sending the Logs. Varies by region [US|EU]"
}

variable "newrelic_api_key" {
  type        = string
  sensitive   = true
  description = "The Ingest API key for sending Logs to New Relic endpoints"
}

variable "newrelic_account_id" {
  type        = string
  sensitive   = true
  description = "The New Relic account ID for sending logging to New Relic endpoints"
}

variable "newrelic_function_app" {
  type        = string
  description = "The name of the function application"
  default     = "newrelic-logging-function-app"
}


variable "connector_hub_name" {
  type        = string
  description = "The prefix for the name of all of the resources"
  default     = "newrelic-logging-connector-hub"
}

variable "function_app_shape" {
  type        = string
  default     = "GENERIC_X86"
  description = "The shape of the function application. The docker image should be built accordingly. Use ARM if using Oracle Resource manager stack"
}

variable "log_group_name" {
  type        = string
  description = "log group name to send logs to New Relic."
}

variable "log_name" {
  type        = string
  description = "log OCID to send logs to New Relic."
}
