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

BIN="bin/verifier"
if [ ! -f "${BIN}" ]; then
    echo "==> Building ${BIN}..."
    go build -o "${BIN}" ./cmd/verifier
fi

ENV_FILE="${ENV_FILE:-.env}"
if [ -f "${ENV_FILE}" ]; then
    echo "==> Loading environment variables from ${ENV_FILE}..."
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
fi

COLLECTOR_ENDPOINT="${OTEL_COLLECTOR_ENDPOINT:-localhost:4317}"
S3_ENDPOINT="${SACLOUD_O2_ENDPOINT:-http://localhost:9000}"
S3_BUCKET="${SACLOUD_O2_BUCKET:-otel-logs}"
S3_ACCESS_KEY="${SACLOUD_O2_ACCESS_KEY:-${AWS_ACCESS_KEY_ID:-minioadmin}}"
S3_SECRET_KEY="${SACLOUD_O2_SECRET_KEY:-${AWS_SECRET_ACCESS_KEY:-minioadmin}}"
S3_PREFIX="${SACLOUD_O2_PREFIX:-logs}"
S3_DISABLE_SSL="${SACLOUD_O2_DISABLE_SSL:-false}"

if [[ "${S3_ENDPOINT}" == http://* ]]; then
    S3_DISABLE_SSL="true"
fi

echo "================================================================================"
echo "  Executing Host-based OTel Verification"
echo "================================================================================"
echo "  • Collector Endpoint : ${COLLECTOR_ENDPOINT}"
echo "  • Storage Endpoint   : ${S3_ENDPOINT}"
echo "  • Bucket / Prefix    : ${S3_BUCKET} / ${S3_PREFIX}"
echo "  • Disable SSL        : ${S3_DISABLE_SSL}"
echo "--------------------------------------------------------------------------------"

"${BIN}" roundtrip \
    --collector-endpoint "${COLLECTOR_ENDPOINT}" \
    --s3-endpoint "${S3_ENDPOINT}" \
    --s3-bucket "${S3_BUCKET}" \
    --s3-prefix "${S3_PREFIX}" \
    --s3-access-key "${S3_ACCESS_KEY}" \
    --s3-secret-key "${S3_SECRET_KEY}" \
    --s3-disable-ssl="${S3_DISABLE_SSL}" \
    --s3-force-path-style=true \
    --count "${LOG_COUNT:-5}" \
    --flush-wait "${FLUSH_WAIT_TIME:-4s}"
