# OCI Log Integration with New Relic - Variables

# Required Configuration
variable "compartment_ocid" {
  description = "OCID of the compartment where resources will be created"
  type        = string
}

variable "tenancy_ocid" {
  description = "OCID of the tenancy"
  type        = string
}

variable "log_group_ocid" {
  description = "OCID of the existing Log Group to forward logs from"
  type        = string
}

variable "log_ocid" {
  description = "OCID of the existing Log to forward logs from"
  type        = string
}


variable "newrelic_ingest_key" {
  description = "New Relic Ingest License Key"
  type        = string
  sensitive   = true
}

# Network Configuration
variable "create_new_vcn" {
  description = "Whether to create a new VCN or use an existing one"
  type        = bool
  default     = true
}

variable "vcn_cidr_block" {
  description = "CIDR block for the VCN (only used if create_new_vcn is true)"
  type        = string
  default     = "10.0.0.0/16"
}

variable "subnet_cidr_block" {
  description = "CIDR block for the subnet (only used if create_new_vcn is true)"
  type        = string
  default     = "10.0.1.0/24"
}

variable "existing_subnet_ocid" {
  description = "OCID of existing subnet to use (only used if create_new_vcn is false)"
  type        = string
  default     = ""
}

# Function Configuration
variable "function_docker_image" {
  description = "Docker image for the log forwarder function (must be accessible from OCI Functions)"
  type        = string
  default     = "iad.ocir.io/idfmbxeaoavl/hrai-container-repo/newrelic-log-forwarder:latest"
}

variable "function_memory_mb" {
  description = "Memory allocation for the function in MB"
  type        = number
  default     = 256
  validation {
    condition     = var.function_memory_mb >= 128 && var.function_memory_mb <= 3008
    error_message = "Function memory must be between 128 and 3008 MB."
  }
}

variable "function_timeout_seconds" {
  description = "Timeout for the function in seconds"
  type        = number
  default     = 30
  validation {
    condition     = var.function_timeout_seconds >= 1 && var.function_timeout_seconds <= 300
    error_message = "Function timeout must be between 1 and 300 seconds."
  }
}

# New Relic Configuration
variable "newrelic_logs_endpoint" {
  description = "New Relic Logs API endpoint URL"
  type        = string
  default     = "https://log-api.newrelic.com/log/v1"
}

# Resource Naming
variable "resource_prefix" {
  description = "Prefix for all resource names"
  type        = string
  default     = "newrelic-log-integration"
}

# Tags
variable "freeform_tags" {
  description = "Freeform tags to apply to all resources"
  type        = map(string)
  default = {
    "Purpose"     = "NewRelicLogIntegration"
    "Environment" = "Production"
  }
}
