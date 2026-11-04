variable "tenancy_ocid" {
  type        = string
  description = "OCI tenant OCID, more details can be found at https://docs.cloud.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five"
}

variable "compartment_ocid" {
  type        = string
  description = "The OCID of the compartment where the repository will be created."
}

variable "region" {
  type        = string
  description = "The name of the OCI region where the repository will be created. Run this stack once per region you deploy the logs-function stack into, not once per tenancy."
}

variable "newrelic_logging_identifier" {
  type        = string
  description = "A unique label or name identifier matching the one used in the logs-function stack. Leave it blank if not needed."
  default     = "logs"
}
