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

remote_image="${dockerhub_repository}"

# A version tag is immutable: re-running a release must never overwrite it, so
# pinned customers always get the same image for a given version.
echo "3. Pushing version tag ${function_build_version}..."
inspect_output="$(docker manifest inspect "${remote_image}:${function_build_version}" 2>&1)"
inspect_status=$?
if [ ${inspect_status} -eq 0 ]; then
    if [ "${REQUIRE_NEW_VERSION:-false}" = "true" ]; then
        # The pushed code changed but the tag is taken: the new code would never be
        # published and customers would keep pulling the old image.
        echo "Error: shipped function files changed but ${function_build_version} already exists on Docker Hub, so the new code would not be released."
        echo "Bump logs-function/VERSION and merge again to release the change under a new version tag."
        exit 1
    fi
    echo "Tag ${function_build_version} already exists on Docker Hub; skipping push and reusing the published image."
    docker pull "${remote_image}:${function_build_version}" || { echo "Error: Failed to pull existing ${function_build_version}."; exit 1; }
elif echo "${inspect_output}" | grep -qiE "no such manifest|manifest unknown"; then
    docker tag "${image_name}:${image_tag}" "${remote_image}:${function_build_version}" || { echo "Error: Docker image tagging failed."; exit 1; }
    docker push "${remote_image}:${function_build_version}" || { echo "Error: Docker image push failed."; exit 1; }
    echo "Successfully pushed ${function_build_version}."
else
    # Could not tell whether the tag exists (auth, network, ...): never risk overwriting it.
    echo "Error: could not check whether ${function_build_version} exists on Docker Hub: ${inspect_output}"
    exit 1
fi

# latest is pushed last, from the published version image, so it only moves once the
# versioned image is in place and always matches it. PUSH_LATEST=false keeps a re-run
# of an older release from moving latest backwards.
if [ "${PUSH_LATEST:-true}" = "true" ]; then
    echo "4. Pushing latest tag..."
    docker tag "${remote_image}:${function_build_version}" "${remote_image}:latest" || { echo "Error: Docker image tagging failed."; exit 1; }
    docker push "${remote_image}:latest" || { echo "Error: Docker image push failed."; exit 1; }
else
    echo "4. Skipping latest tag: ${function_build_version} is not the newest version on main."
fi

echo "5. Reading back the pushed digest for ${dockerhub_repository}:${function_build_version}..."
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
