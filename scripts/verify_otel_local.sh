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

COMPOSE_FILE="deploy/docker-compose.otel-s3.yml"
BIN="bin/verifier"

if [ ! -f "${BIN}" ]; then
    echo "==> Building ${BIN}..."
    go build -o "${BIN}" ./cmd/verifier
fi

echo "==> Starting local MinIO and sacloud-otel-collector containers..."
docker compose -f "${COMPOSE_FILE}" up -d --wait

cleanup() {
    echo "==> Stopping local verification containers..."
    docker compose -f "${COMPOSE_FILE}" down -v
}
trap cleanup EXIT

echo "==> Waiting for collector OTLP receiver (localhost:4317)..."
for i in {1..30}; do
    if nc -z localhost 4317 2>/dev/null || (echo > /dev/tcp/localhost/4317) 2>/dev/null; then
        echo "    ✓ Collector is ready on port 4317!"
        break
    fi
    sleep 1
done

echo "==> Executing OTel ➔ MinIO (S3 emulator) roundtrip verification..."
"${BIN}" roundtrip \
    --collector-endpoint "localhost:4317" \
    --s3-endpoint "http://localhost:9000" \
    --s3-access-key "minioadmin" \
    --s3-secret-key "minioadmin" \
    --s3-bucket "otel-logs" \
    --s3-prefix "logs" \
    --s3-disable-ssl=true \
    --s3-force-path-style=true \
    --count 5 \
    --flush-wait 3s

echo "✅ Local OTel ➔ S3 roundtrip verification succeeded!"
