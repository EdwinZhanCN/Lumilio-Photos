#!/usr/bin/env bash

set -euo pipefail

: "${GITHUB_EVENT_NAME:?GITHUB_EVENT_NAME is required}"
: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"
: "${GITHUB_RUN_NUMBER:?GITHUB_RUN_NUMBER is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"

short_sha="${GITHUB_SHA:0:12}"
is_release=false
is_prerelease=true
channel=edge
tag_name=

if [[ "${GITHUB_EVENT_NAME}" == "push" ]]; then
    if [[ "${GITHUB_REF_TYPE:-}" != "tag" ]]; then
        echo "release pushes must be tag events" >&2
        exit 1
    fi
    if [[ ! "${GITHUB_REF_NAME}" =~ ^v([0-9]{2})\.([1-9][0-9]*)\.(0|[1-9][0-9]*)(-(beta|rc)\.([1-9][0-9]*))?$ ]]; then
        echo "invalid release tag ${GITHUB_REF_NAME}; expected vYY.TRAIN.PATCH[-beta.N|-rc.N]" >&2
        exit 1
    fi

    product_version="${GITHUB_REF_NAME#v}"
    platform_version="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
    artifact_ref="${GITHUB_REF_NAME}"
    image_tag="${product_version}"
    tag_name="${GITHUB_REF_NAME}"
    is_release=true
    if [[ "${product_version}" != *-* ]]; then
        is_prerelease=false
        channel=stable
    else
        channel="${BASH_REMATCH[5]}"
    fi
elif [[ "${GITHUB_EVENT_NAME}" == "workflow_dispatch" ]]; then
    product_version="0.0.0-edge.${GITHUB_RUN_NUMBER}+${short_sha}"
    platform_version="0.0.0"
    artifact_ref="edge-${short_sha}"
    image_tag=edge
else
    echo "unsupported release event ${GITHUB_EVENT_NAME}" >&2
    exit 1
fi

{
    echo "product_version=${product_version}"
    echo "platform_version=${platform_version}"
    echo "build_number=${GITHUB_RUN_NUMBER}"
    echo "artifact_ref=${artifact_ref}"
    echo "image_tag=${image_tag}"
    echo "tag_name=${tag_name}"
    echo "channel=${channel}"
    echo "is_release=${is_release}"
    echo "is_prerelease=${is_prerelease}"
    echo "short_sha=${short_sha}"
} >> "${GITHUB_OUTPUT}"
