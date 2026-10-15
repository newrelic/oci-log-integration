variable "tenancy_ocid" {
  type        = string
  description = "OCI tenant OCID, more details can be found at https://docs.cloud.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five"
}

variable "compartment_ocid" {
  type        = string
  description = "The OCID of the compartment where resources will be created. Do not modify."
}

variable "newrelic_logging_identifier" {
  type        = string
  description = "A unique label or name identifier for all resources in this deployment. Leave it blank if not needed."
  default     = "logs"
}

variable "region" {
  type        = string
  description = "The name of the OCI region where these resources will be deployed."
}

variable "new_relic_region" {
  type        = string
  default     = "US"
  description = "New Relic Region. US, EU, or JP"
}

variable "newrelic_account_id" {
  type        = string
  sensitive   = true
  description = "The New Relic account ID for sending metrics to New Relic endpoints"
}

variable "create_vcn" {
  type        = bool
  default     = true
  description = "Variable to create virtual network for the setup. True by default"
}

variable "function_subnet_id" {
  type        = string
  default     = ""
  description = "The OCID of the subnet to be used for the function app. If create_vcn is set to true, that will take precedence"
}

variable "payload_link" {
  type        = string
  description = "The link to the payload for the connector hubs."
  default     = ""
}

variable "debug_enabled" {
  type        = string
  default     = "FALSE"
  description = "Enable debug mode."
}

variable "image_version" {
  type        = string
  description = "Deprecated, no longer used. Superseded by function_image, which now carries the full image reference (registry/repository:tag) instead of just a tag applied to a hardcoded New Relic-owned OCIR path."
  default     = "latest"
}

variable "function_image" {
  type        = string
  default     = "docker.io/newrelic/oci-log-forwarder:latest"
  description = "Public image for the log-forwarder function. The stack copies it into a private Container Registry repository in your tenancy and runs the function from there. Re-applying the stack picks up a new image pushed under the same tag."
}

variable "current_user_ocid" {
  type        = string
  default     = ""
  description = "OCID of the user running the stack. Populated by Resource Manager; used to create the auth token that pushes the function image to Container Registry."
}

variable "registry_username" {
  type        = string
  default     = ""
  description = "Container Registry username, without the tenancy namespace. Leave empty to use the user running the stack. Set it for users in a non-default identity domain (/)."
}

variable "registry_auth_token" {
  type        = string
  default     = ""
  sensitive   = true
  description = "Existing auth token for pushing to Container Registry. Leave empty to have the stack create one for the user running it (OCI allows two auth tokens per user)."
}

variable "metrics_tier" {
  type        = string
  default     = "basic"
  description = "Tier of custom metrics the function emits about itself (in addition to New Relic's own ingested logs): none (no custom metrics), basic (core health metrics: invocations, records received/delivered/dropped, delivery duration, pipeline lag), or advanced (basic plus deeper root-cause/tuning metrics: byte volumes, decode/serialize errors, batching behavior, delivery error classes, run duration, secret-fetch failures, client-cache hit rate). Custom metrics are billed by New Relic on ingest; basic is the default, set to none to opt out."

  validation {
    condition     = contains(["none", "basic", "advanced"], var.metrics_tier)
    error_message = "metrics_tier must be one of: none, basic, advanced."
  }
}
