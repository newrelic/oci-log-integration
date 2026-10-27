output "container_repository_id" {
  description = "OCID of the created Container Registry repository. Select this stack's repository in the logs-function stack's \"Container Registry repository\" field."
  value       = oci_artifacts_container_repository.log_forwarder_repo.id
}

output "container_repository_namespace" {
  description = "OCIR namespace the repository was created under."
  value       = oci_artifacts_container_repository.log_forwarder_repo.namespace
}
