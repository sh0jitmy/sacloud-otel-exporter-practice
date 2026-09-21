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

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$ROOT_DIR/bin"

mkdir -p "$ROOT_DIR/data" "$ROOT_DIR/data/backups"

echo "==> Building binaries (bin/app, bin/web)..."
make -C "$ROOT_DIR" build

echo "==> Starting local standalone stack (Mac / Linux / Windows WSL)..."

WEB_PORT=3001

# Auto-detect available core server port (default 8080, fallback to 18080 if 8080 is in use)
if [ -n "${CORE_PORT:-}" ]; then
    :
elif lsof -i :8080 >/dev/null 2>&1; then
    CORE_PORT=18080
    echo "  (Note: Port 8080 is currently in use. Using port $CORE_PORT for core API server)"
else
    CORE_PORT=8080
fi

# Clean shutdown handler
cleanup() {
    trap - SIGINT SIGTERM EXIT
    echo ""
    echo "==> Stopping local standalone stack..."
    [ -n "${WEB_PID:-}" ] && kill "$WEB_PID" 2>/dev/null || true
    [ -n "${CORE_PID:-}" ] && kill "$CORE_PID" 2>/dev/null || true
    pkill -P $$ 2>/dev/null || true
    echo "==> All processes stopped."
    exit 0
}
trap cleanup SIGINT SIGTERM EXIT

# 1. Start Core API Server (SQLite Standalone)
echo "  [1/2] Starting Core API Server on port $CORE_PORT..."
PORT=$CORE_PORT \
DATABASE_DRIVER=sqlite3 \
DATABASE_DSN="file:$ROOT_DIR/data/app.db?cache=shared&mode=rwc&_pragma=foreign_keys(1)" \
BACKUP_DIR="$ROOT_DIR/data/backups" \
"$BIN_DIR/app" server --port "$CORE_PORT" > "$ROOT_DIR/data/server.log" 2>&1 &
CORE_PID=$!

sleep 0.5
if ! kill -0 $CORE_PID 2>/dev/null; then
    echo "Error: app server failed to start. Logs:"
    cat "$ROOT_DIR/data/server.log"
    exit 1
fi

# Wait for Core Server health
for i in {1..20}; do
    if curl -sf "http://127.0.0.1:$CORE_PORT/v1/system/healthz" > /dev/null 2>&1; then
        break
    fi
    sleep 0.5
done

# 2. Start HTMX Web Dashboard
echo "  [2/2] Starting HTMX Web Dashboard on port $WEB_PORT..."
PORT=$WEB_PORT \
DATABASE_DRIVER=sqlite3 \
DATABASE_DSN="file:$ROOT_DIR/data/app.db?cache=shared&mode=rwc&_pragma=foreign_keys(1)" \
BACKUP_DIR="$ROOT_DIR/data/backups" \
"$BIN_DIR/web" --port "$WEB_PORT" > "$ROOT_DIR/data/web.log" 2>&1 &
WEB_PID=$!

sleep 1

echo ""
echo "=========================================================================="
echo " ⚡ Go Template Standalone Local Stack is ONLINE!"
echo "   🌐 Web Dashboard:    http://localhost:$WEB_PORT"
echo "   🔌 Core REST API:    http://localhost:$CORE_PORT/v1/system/healthz"
echo "   📊 Prometheus:       http://localhost:$CORE_PORT/metrics"
echo "   🛡️ Readyz:           http://localhost:$CORE_PORT/v1/system/readyz"
echo "=========================================================================="
echo "Press Ctrl+C to stop all services."
echo ""

wait $WEB_PID
