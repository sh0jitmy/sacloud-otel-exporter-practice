#!/usr/bin/env bash
# Copyright 2026 [Copyright Holder]
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Author: [YOUR_NAME]

set -euo pipefail

VERSION="${COLLECTOR_VERSION:-0.8.0}"
BIN_DIR="${1:-bin}"
mkdir -p "${BIN_DIR}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${ARCH}" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "❌ Unsupported architecture: ${ARCH}" >&2
        exit 1
        ;;
esac

ASSET="sacloud-otel-collector_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/sacloud/sacloud-otel-collector/releases/download/v${VERSION}/${ASSET}"

echo "==> Downloading sacloud-otel-collector v${VERSION} for ${OS}/${ARCH}..."
echo "    URL: ${URL}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -sSL "${URL}" -o "${TMP_DIR}/${ASSET}"
tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}"

mv "${TMP_DIR}/sacloud-otel-collector" "${BIN_DIR}/sacloud-otel-collector"
chmod +x "${BIN_DIR}/sacloud-otel-collector"

echo "✅ Installed sacloud-otel-collector to ${BIN_DIR}/sacloud-otel-collector"
"${BIN_DIR}/sacloud-otel-collector" --version
