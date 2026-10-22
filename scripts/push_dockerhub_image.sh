#!/bin/bash

# --- Configuration Variables ---
dockerhub_username="${DOCKERHUB_USERNAME}"
dockerhub_token="${DOCKERHUB_TOKEN}"
function_build_version="${FUNCTION_BUILD_VERSION}"

dockerhub_repository="${DOCKERHUB_REPOSITORY:-newrelic/oci-log-forwarder}"
image_name="${IMAGE_NAME:-oci-log-forwarder}"
image_tag="${IMAGE_TAG:-latest}"

if [ -z "${dockerhub_username}" ] || [ -z "${dockerhub_token}" ]; then
  echo "Error: DOCKERHUB_USERNAME and DOCKERHUB_TOKEN environment variables must be set."
  exit 1
fi
if [ -z "${function_build_version}" ]; then
  echo "Error: FUNCTION_BUILD_VERSION environment variable is not set."
  exit 1
fi

echo "--- Starting Docker Hub Publish (Version: ${function_build_version}) ---"

echo "1. Building Docker image..."
# Functions run on GENERIC_X86 (linux/amd64), and customer stacks copy this image as-is.
# provenance/sbom disabled so the pushed image stays a single plain manifest --
# the mirror's change-check (image_mirror.py) compares that one manifest's digest.
docker build --platform linux/amd64 --provenance=false --sbom=false \
  -t "${image_name}:${image_tag}" logs-function/

if [ $? -ne 0 ]; then
    echo "Error: Docker image build failed."
    exit 1
fi

echo "2. Logging in to Docker Hub..."
echo "${dockerhub_token}" | docker login -u "${dockerhub_username}" --password-stdin

if [ $? -ne 0 ]; then
    echo "Error: Docker Hub login failed."
    exit 1
fi

echo "3. Tagging and pushing ${dockerhub_repository}:${function_build_version} and :latest..."
for tag in "${function_build_version}" latest; do
    docker tag "${image_name}:${image_tag}" "${dockerhub_repository}:${tag}"
    if [ $? -ne 0 ]; then
        echo "Error: Docker image tagging failed for tag ${tag}."
        exit 1
    fi

    docker push "${dockerhub_repository}:${tag}"
    if [ $? -ne 0 ]; then
        echo "Error: Docker image push failed for tag ${tag}."
        exit 1
    fi
done

echo "4. Reading back the pushed digest for ${dockerhub_repository}:${function_build_version}..."
# docker updates RepoDigests on the local image record once a push succeeds, so this
# reads the digest the registry actually stored -- not a locally-computed guess.
# RepoDigests' repo portion isn't guaranteed to include the docker.io/ host, so pull
# just the @sha256:... suffix and rebuild the reference explicitly.
digest_only=$(docker inspect --format='{{index .RepoDigests 0}}' "${dockerhub_repository}:${function_build_version}" | sed -E 's#^.*(@sha256:[0-9a-f]+)$#\1#')
if [ -z "${digest_only}" ]; then
    echo "Error: Could not read back a pushed digest for ${dockerhub_repository}:${function_build_version}."
    exit 1
fi
digest_ref="docker.io/${dockerhub_repository}${digest_only}"
echo "Pushed digest: ${digest_ref}"
if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "image_digest_ref=${digest_ref}" >> "${GITHUB_OUTPUT}"
fi

echo "--- Docker Hub Publish Completed Successfully ---"
