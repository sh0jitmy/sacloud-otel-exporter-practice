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

ENV_FILE="${ENV_FILE:-.env}"
CONFIG_FILE="${1:-deploy/sacloud-otel-collector/config.sakura.yaml}"
LOG_DIR="test_reports"
LOG_FILE="${LOG_DIR}/collector.log"
mkdir -p "${LOG_DIR}"

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

# Ensure binaries exist
if [ ! -f "bin/verifier" ]; then
    echo "==> [2/4] Building bin/verifier..."
    go build -o bin/verifier ./cmd/verifier
fi

if [ ! -f "bin/sacloud-otel-collector" ]; then
    echo "==> [2/4] Installing sacloud-otel-collector..."
    ./scripts/install_collector_host.sh bin
fi

echo "==> [3/4] Starting sacloud-otel-collector in background (log: ${LOG_FILE})..."
./scripts/run_collector_host.sh "${CONFIG_FILE}" bin/sacloud-otel-collector > "${LOG_FILE}" 2>&1 &
COLLECTOR_PID=$!

cleanup() {
    status=$?
    echo ""
    echo "==> Stopping background collector (PID: ${COLLECTOR_PID})..."
    kill "${COLLECTOR_PID}" 2>/dev/null || true
    wait "${COLLECTOR_PID}" 2>/dev/null || true
    if [ $status -ne 0 ] && [ -f "${LOG_FILE}" ]; then
        echo "--- Collector Log tail (last 15 lines) ---"
        tail -n 15 "${LOG_FILE}" || true
        echo "------------------------------------------"
    fi
    exit $status
}
trap cleanup EXIT INT TERM

# Wait for port 4317 to be open
echo "    ⏳ Waiting for collector OTLP receiver (localhost:4317)..."
READY=false
for _ in {1..30}; do
    if nc -z localhost 4317 2>/dev/null || (echo > /dev/tcp/localhost/4317) 2>/dev/null; then
        READY=true
        echo "    ✓ Collector is ready on port 4317!"
        break
    fi
    if ! kill -0 "${COLLECTOR_PID}" 2>/dev/null; then
        echo "❌ Collector process exited unexpectedly. Log output:"
        cat "${LOG_FILE}"
        exit 1
    fi
    sleep 0.5
done

if [ "${READY}" = "false" ]; then
    echo "❌ Timed out waiting for collector to listen on port 4317."
    cat "${LOG_FILE}"
    exit 1
fi

echo "==> [4/4] Executing verification software (bin/verifier roundtrip)..."
./bin/verifier roundtrip --env-file "${ENV_FILE}"

echo "🎉 All-in-one verification finished successfully!"
