#!/bin/bash

# --- Configuration Variables ---
oci_auth_token="${ORACLE_AUTH_TOKEN}"
REGION="$1"

tenancy_namespace="${OCI_TENANCY_NAMESPACE}"
repository_name="${REPOSITORY_NAME:-newrelic-logs-integration/oci-log-forwarder}"
image_name="${IMAGE_NAME:-oci-log-forwarder}"
image_tag="${IMAGE_TAG:-$(tr -d '[:space:]' < "$(dirname "$0")/../VERSION")}"
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
docker build -t "${image_name}:${image_tag}" -t "${image_name}:latest" logs-function/

if [ $? -ne 0 ]; then
    echo "Error: Docker image build failed."
    exit 1
fi

if [ -z "${REGION}" ]; then
  echo "Error: Region is required for push operations."
  exit 1
fi

remote_image="${REGION}.ocir.io/${tenancy_namespace}/${repository_name}"

echo "2. Logging in to OCI Container Registry: ${REGION}.ocir.io..."
echo "${oci_auth_token}" | docker login "${REGION}.ocir.io" -u "${tenancy_namespace}/${username}" --password-stdin

if [ $? -ne 0 ]; then
    echo "Error: Docker login to OCIR failed."
    exit 1
fi
echo "Successfully logged in to OCIR."

# A version tag is immutable: re-running a release must never overwrite it, so
# pinned customers always get the same image for a given version.
echo "3. Pushing version tag ${image_tag}..."
if docker manifest inspect "${remote_image}:${image_tag}" > /dev/null 2>&1; then
    echo "Tag ${image_tag} already exists in ${REGION}; skipping push and reusing the published image."
    docker pull "${remote_image}:${image_tag}" || { echo "Error: Failed to pull existing ${image_tag}."; exit 1; }
else
    docker tag "${image_name}:${image_tag}" "${remote_image}:${image_tag}" || { echo "Error: Docker image tagging failed."; exit 1; }
    docker push "${remote_image}:${image_tag}" || { echo "Error: Docker image push failed."; exit 1; }
    echo "Successfully pushed ${image_tag}."
fi

# latest is pushed last, from the published version image, so it only moves once
# the versioned image is in place and always matches it.
echo "4. Pushing latest tag..."
docker tag "${remote_image}:${image_tag}" "${remote_image}:latest" || { echo "Error: Docker image tagging failed."; exit 1; }
docker push "${remote_image}:latest" || { echo "Error: Docker image push failed."; exit 1; }
echo "Successfully pushed Docker image to OCIR."

echo "--- Docker Image Build and Push Automation Completed Successfully ---"
