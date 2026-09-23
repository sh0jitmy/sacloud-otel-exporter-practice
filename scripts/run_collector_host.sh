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

CONFIG_FILE="${1:-deploy/sacloud-otel-collector/config.local.yaml}"
BIN="${2:-bin/sacloud-otel-collector}"

if [ ! -f "${BIN}" ]; then
    echo "==> ${BIN} not found. Running installer..."
    ./scripts/install_collector_host.sh "$(dirname "${BIN}")"
fi

ENV_FILE="${ENV_FILE:-.env}"
if [ -f "${ENV_FILE}" ]; then
    echo "==> Loading environment variables from ${ENV_FILE}..."
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
fi

# Fallback defaults
export SACLOUD_O2_ENDPOINT="${SACLOUD_O2_ENDPOINT:-https://s3.tky01.sakurastorage.jp}"
export SACLOUD_O2_REGION="${SACLOUD_O2_REGION:-jp-north-1}"
export SACLOUD_O2_BUCKET="${SACLOUD_O2_BUCKET:-otel-logs}"
export SACLOUD_O2_PREFIX="${SACLOUD_O2_PREFIX:-logs}"
export S3_ENDPOINT="${S3_ENDPOINT:-http://localhost:9000}"

# Map Sakura Object Storage keys to standard AWS SDK credential variables expected by awss3exporter
if [ -n "${SACLOUD_O2_ACCESS_KEY:-}" ] && [ -z "${AWS_ACCESS_KEY_ID:-}" ]; then
    export AWS_ACCESS_KEY_ID="${SACLOUD_O2_ACCESS_KEY}"
fi
if [ -n "${SACLOUD_O2_SECRET_KEY:-}" ] && [ -z "${AWS_SECRET_ACCESS_KEY:-}" ]; then
    export AWS_SECRET_ACCESS_KEY="${SACLOUD_O2_SECRET_KEY}"
fi

# Critical compatibility flags for AWS SDK v2 with Sakura Cloud Object Storage and S3 emulators
export AWS_RESPONSE_CHECKSUM_VALIDATION="${AWS_RESPONSE_CHECKSUM_VALIDATION:-WHEN_REQUIRED}"
export AWS_REQUEST_CHECKSUM_CALCULATION="${AWS_REQUEST_CHECKSUM_CALCULATION:-WHEN_REQUIRED}"

echo "================================================================================"
echo "  Starting sacloud-otel-collector on Host"
echo "================================================================================"
echo "  • Binary : ${BIN}"
echo "  • Config : ${CONFIG_FILE}"
echo "  • Checksum flags: AWS_RESPONSE_CHECKSUM_VALIDATION=${AWS_RESPONSE_CHECKSUM_VALIDATION}"
echo "                    AWS_REQUEST_CHECKSUM_CALCULATION=${AWS_REQUEST_CHECKSUM_CALCULATION}"
echo "--------------------------------------------------------------------------------"

exec "${BIN}" --config "${CONFIG_FILE}"
