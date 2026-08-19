#!/usr/bin/env bash
# Credits: https://github.com/vmware-tanzu/velero/blob/main/hack/build.sh

set -o errexit
set -o nounset
set -o pipefail

if [[ -z "${PKG}" ]]; then
    echo "PKG must be set"
    exit 1
fi
if [[ -z "${BIN}" ]]; then
    echo "BIN must be set"
    exit 1
fi
if [[ -z "${GOOS}" ]]; then
    echo "GOOS must be set"
    exit 1
fi
if [[ -z "${GOARCH}" ]]; then
    echo "GOARCH must be set"
    exit 1
fi
if [[ -z "${VERSION}" ]]; then
    echo "VERSION must be set"
    exit 1
fi

GCFLAGS=""
if [[ ${DEBUG:-} = "1" ]]; then
    GCFLAGS="all=-N -l"
fi

export CGO_ENABLED=1

LDFLAGS="-s -w"

if [[ -z "${OUTPUT_DIR:-}" ]]; then
  OUTPUT_DIR=.
fi

OUTPUT=${OUTPUT_DIR}/${BIN}

go build \
    -o "${OUTPUT}" \
    -gcflags "${GCFLAGS}" \
    -installsuffix "static" \
    -ldflags "${LDFLAGS}" \
    "${PKG}/cmd/${BIN}"
