#!/bin/bash

# --- Configuration Variables ---

image_name="${IMAGE_NAME:-oci-log-forwarder}"
image_tag="${IMAGE_TAG:-$(tr -d '[:space:]' < "$(dirname "$0")/../logs-function/VERSION" 2>/dev/null)}"

if [ -z "${image_tag}" ]; then
  echo "Error: could not read the function version from logs-function/VERSION."
  exit 1
fi


echo "--- Starting Docker Image Build ---"

# --- Build Phase ---
echo "1. Building Docker image..."
docker build -t "${image_name}:${image_tag}" -t "${image_name}:latest" logs-function/

if [ $? -ne 0 ]; then
    echo "Error: Docker image build failed."
    exit 1
fi

echo "Successfully build Docker image "
