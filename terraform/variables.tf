variable "tenancy_ocid" {
  type        = string
  description = "OCI tenant OCID, more details can be found at https://docs.cloud.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five"
}

variable "current_user_ocid" {
  type        = string
  description = "The OCID of the current user executing the terraform script. Do not modify."
}

variable "newrelic_region" {
  type        = string
  description = "OCI Region as documented at https://docs.cloud.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm"
}

variable "newrelic_dynamic_group_name" {
  type        = string
  description = "The name of the dynamic group for giving access to service connector"
  default     = "newrelic-dynamic-group"
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

variable "newrelic_function_app" {
  type        = string
  description = "The name of the function application"
  default     = "newrelic-function-app"
}


variable "newrelic_connector_hub_name" {
  type        = string
  description = "The prefix for the name of all of the resources"
  default     = "newrelic-connector-hub"
}

variable "function_app_shape" {
  type        = string
  default     = "GENERIC_X86"
  description = "The shape of the function application. The docker image should be built accordingly. Use ARM if using Oracle Resource manager stack"
}

variable "debug_enabled" {
  type        = string
  default     = "FALSE"
  description = "Enable debug mode."
}

# variables.tf
variable "label_prefix" {
  type        = string
  description = "Prefix for resource names in OCI to ensure uniqueness across deployments"
  default     = "newrelic-logging"

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.label_prefix))
    error_message = "Label prefix must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "log_sources" {
  type        = string
  description = "JSON string representing a list of log sources to onboard. Each can be from different compartments."

  # validation to ensure at least one log_sources data is present
  validation {
    condition     = can(jsondecode(var.log_sources)) && length(jsondecode(var.log_sources)) > 0
    error_message = "log_sources must be valid JSON with at least one log source."
  }

  # validation to ensure log_sources json is correct 
  validation {
    condition = alltrue([
      for source in jsondecode(var.log_sources) :
      can(source.compartment_name) && can(source.log_group_name) && can(source.log_name)
    ])
    error_message = "Each log source must have compartment_name, log_group_name and log_name"
  }
}
