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

MODE="${1:-sakura}"
ENV_FILE="${ENV_FILE:-.env}"
COMPOSE_FILE="deploy/docker-compose.docker-log.yml"
TEST_RUN_ID="docker-${MODE}-$(date +%Y%m%d-%H%M%S)-$RANDOM"
LOG_COUNT="${LOG_COUNT:-5}"

echo "================================================================================"
echo "  Docker Container Log Verification (${MODE} mode)"
echo "================================================================================"
echo "  • Run ID     : ${TEST_RUN_ID}"
echo "  • Log Count  : ${LOG_COUNT}"
echo "  • Mode       : ${MODE}"
echo "--------------------------------------------------------------------------------"

# Build verifier if not already present
if [ ! -f "bin/verifier" ]; then
    echo "==> Building bin/verifier..."
    go build -o bin/verifier ./cmd/verifier
fi

if [ "${MODE}" = "sakura" ]; then
    if [ ! -f "${ENV_FILE}" ]; then
        echo "❌ Error: ${ENV_FILE} not found."
        echo "   Please create .env first: cp .env.example .env (and configure your credentials)"
        exit 1
    fi
    echo "==> [1/4] Loading environment variables from ${ENV_FILE}..."
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a

    export TEST_RUN_ID
    export LOG_COUNT

    cleanup() {
        status=$?
        echo ""
        echo "==> [Cleanup] Stopping Docker Compose stack..."
        docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" down -v --remove-orphans 2>/dev/null || true
        exit $status
    }
    trap cleanup EXIT INT TERM

    echo "==> [2/4] Starting Docker Compose stack (log-producer + collector)..."
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" down -v --remove-orphans 2>/dev/null || true
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d

    echo "==> [3/4] Waiting 6s for logs to be produced and flushed by collector to Sakura Object Storage..."
    sleep 6

    echo "==> [4/4] Verifying logs in Sakura Cloud Object Storage..."
    ./bin/verifier verify \
        --run-id "${TEST_RUN_ID}" \
        --expected-count "${LOG_COUNT}" \
        --env-file "${ENV_FILE}"

elif [ "${MODE}" = "local" ]; then
    LOCAL_COMPOSE_FILE="deploy/docker-compose.docker-log-local.yml"
    export TEST_RUN_ID
    export LOG_COUNT

    cleanup_local() {
        status=$?
        echo ""
        echo "==> [Cleanup] Stopping local Docker Compose stack..."
        docker compose -f "${LOCAL_COMPOSE_FILE}" down -v --remove-orphans 2>/dev/null || true
        exit $status
    }
    trap cleanup_local EXIT INT TERM

    echo "==> [1/4] Starting local Docker Compose stack (minio + log-producer + collector)..."
    docker compose -f "${LOCAL_COMPOSE_FILE}" down -v --remove-orphans 2>/dev/null || true
    docker compose -f "${LOCAL_COMPOSE_FILE}" up -d

    echo "==> [2/4] Waiting 8s for MinIO initialization and collector batch flush..."
    sleep 8

    echo "==> [3/4] Verifying logs in local MinIO storage..."
    ./bin/verifier verify \
        --run-id "${TEST_RUN_ID}" \
        --expected-count "${LOG_COUNT}" \
        --s3-endpoint "http://localhost:9000" \
        --s3-region "jp-north-1" \
        --s3-bucket "otel-logs" \
        --s3-prefix "logs" \
        --s3-access-key "minioadmin" \
        --s3-secret-key "minioadmin" \
        --s3-disable-ssl \
        --s3-force-path-style

else
    echo "❌ Unknown mode: ${MODE}. Expected 'sakura' or 'local'."
    exit 1
fi

echo "🎉 Docker container log verification completed successfully!"
