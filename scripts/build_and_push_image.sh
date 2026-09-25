#!/bin/bash

# --- Configuration Variables ---
oci_auth_token="${ORACLE_AUTH_TOKEN}"
REGION="$1"

tenancy_namespace="${OCI_TENANCY_NAMESPACE}"
repository_name="${REPOSITORY_NAME:-newrelic-logs-integration/oci-log-forwarder}"
image_name="${IMAGE_NAME:-oci-log-forwarder}"
image_tag="${IMAGE_TAG:-latest}"
username="${OCI_USERNAME}"


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

echo "--- Starting Docker Image Build and Push Automation for  Region: ${REGION} ---"

# --- Build Phase ---
echo "1. Building Docker image..."
docker build -t "${image_name}:${image_tag}" logs-function/

if [ $? -ne 0 ]; then
    echo "Error: Docker image build failed."
    exit 1
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

remote_tag="${REGION}.ocir.io/${tenancy_namespace}/${repository_name}:${image_tag}"

if docker manifest inspect "${remote_tag}" >/dev/null 2>&1; then
    echo "Error: Tag '${image_tag}' already exists in ${REGION}.ocir.io. Bump logs-function/VERSION before releasing again."
    exit 1
fi

echo "4. Pushing Docker image..."
docker push "${remote_tag}"
if [ $? -ne 0 ]; then
    echo "Error: Docker image push failed."
    exit 1
fi
echo "Successfully pushed Docker image to OCIR."

echo "5. Updating floating 'latest' tag..."
latest_remote_tag="${REGION}.ocir.io/${tenancy_namespace}/${repository_name}:latest"
docker tag "${image_name}:${image_tag}" "${latest_remote_tag}"
docker push "${latest_remote_tag}"
if [ $? -ne 0 ]; then
    echo "Error: Docker image push for 'latest' tag failed."
    exit 1
fi
echo "Successfully updated 'latest' tag in OCIR."

echo "--- Docker Image Build and Push Automation Completed Successfully ---"