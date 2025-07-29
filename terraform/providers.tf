terraform {
  required_version = ">= 1.2.0"
  required_providers {
    oci = {
      source  = "hashicorp/oci"
      version = "7.11.0"
    }
    newrelic = {
      source  = "newrelic/newrelic"
    }
  }
}

provider "oci" {
  alias        = "home"
  tenancy_ocid = var.tenancy_ocid
  user_ocid    = data.oci_identity_user.current_user.user_id
  region       = var.region
}

data "oci_identity_user" "current_user" {
  user_id = var.current_user_ocid
}