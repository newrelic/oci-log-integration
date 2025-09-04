#!/bin/bash

# --- Configuration Variables ---
oci_auth_token="${ORACLE_AUTH_TOKEN}"
function_build_version="${FUNCTION_BUILD_VERSION}"
MODE=${1:-"full"} # "build-only" or "full"
REGION=${2:-"us-ashburn-1"}

tenancy_namespace="${OCI_TENANCY_NAMESPACE}"
repository_name="${REPOSITORY_NAME:-hrai-container-repo/newrelic-log-forwarder}"
image_name="${IMAGE_NAME:-oci-logs-function}"
image_tag="${IMAGE_TAG:-latest}"
username="${OCI_USERNAME}"

echo "FUNCTION_BUILD_VERSION: ${function_build_version}"

if [ -z "${oci_auth_token}" ]; then
  echo "Error: ORACLE_AUTH_TOKEN environment variable is not set."
  exit 1
fi
if [ -z "${tenancy_namespace}" ]; then
  echo "Error: OCI_TENANCY_NAMESPACE environment variable is not set."
  exit 1
fi
if [ -z "${username}" ]; then
  echo "Error: OCI_USERNAME environment variable is not set."
  exit 1
fi

echo "--- Starting Docker Image Build and Push Automation (Mode: ${MODE}, Region: ${REGION}, Version: ${function_build_version}) ---"

# --- Build Phase ---
echo "1. Building Docker image..."
docker build --build-arg FUNCTION_BUILD_VERSION="${function_build_version}" -t "${image_name}:${image_tag}" logs-function/

if [ $? -ne 0 ]; then
    echo "Error: Docker image build failed."
    exit 1
fi

if [ "${MODE}" == "build-only" ]; then
    echo "Build-only mode: Image built successfully, skipping tag and push."
    exit 0
fi

if [ -z "${REGION}" ]; then
  echo "Error: Region is required for push operations."
  exit 1
fi

echo "2. Tagging Docker image..."
docker tag "${image_name}:${image_tag}" "${REGION}.ocir.io/${tenancy_namespace}/${repository_name}:${image_tag}"

if [ $? -ne 0 ]; then
    echo "Error: Docker image tagging failed."
    exit 1
fi

echo "3. Logging in to OCI Container Registry: ${REGION}.ocir.io..."
echo "${oci_auth_token}" | docker login "${REGION}.ocir.io" -u "${tenancy_namespace}/${username}" --password-stdin

if [ $? -ne 0 ]; then
    echo "Error: Docker login to OCIR failed."
    exit 1
fi
echo "Successfully logged in to OCIR."

echo "4. Pushing Docker image..."
docker push "${REGION}.ocir.io/${tenancy_namespace}/${repository_name}:${image_tag}"

if [ $? -ne 0 ]; then
    echo "Error: Docker image push failed."
    exit 1
fi
echo "Successfully pushed Docker image to OCIR."

echo "--- Docker Image Build and Push Automation Completed Successfully ---"