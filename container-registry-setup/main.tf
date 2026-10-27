terraform {
  required_version = ">= 1.2.0"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "7.12.0"
    }
  }
}

provider "oci" {
  tenancy_ocid = var.tenancy_ocid
  region       = var.region
}

# OCIR repository names are unique tenancy+region-wide, not scoped per compartment -- a
# repository created inside the main logs-function stack itself collides with any other
# deployment in the same tenancy+region (NR-640865). This is a separate, one-time Resource
# Manager stack for exactly that reason: run it once per region before the logs-function
# stack, and every logs-function deployment in that region selects this stack's repository
# instead of creating its own.
resource "oci_artifacts_container_repository" "log_forwarder_repo" {
  compartment_id = var.compartment_ocid
  display_name   = "newrelic-${var.newrelic_logging_identifier}/oci-log-forwarder"
  is_public      = false
  freeform_tags = {
    newrelic-terraform = "true"
  }
}
