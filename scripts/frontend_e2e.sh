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

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${CYAN}================================================================${NC}"
echo -e "${CYAN}   Go Template HTMX Frontend (No-Docker) E2E Test Suite         ${NC}"
echo -e "${CYAN}================================================================${NC}"

WORK_DIR=$(mktemp -d -t go-template-frontend-e2e-XXXXXX)
BACKUP_DIR="$WORK_DIR/backups"
mkdir -p "$BACKUP_DIR"

REPORT_DIR="$(pwd)/test_reports"
mkdir -p "$REPORT_DIR"
mkdir -p "docs/images"

SERVER_PORT=18080
WEB_PORT=18081

SERVER_PID=""
WEB_PID=""

cleanup() {
    echo -e "\n${YELLOW}===> [Frontend E2E Cleanup] Stopping processes and cleaning temporary directory...${NC}"
    if [ -n "$WEB_PID" ] && kill -0 "$WEB_PID" > /dev/null 2>&1; then
        kill "$WEB_PID" > /dev/null 2>&1 || true
    fi
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" > /dev/null 2>&1; then
        kill "$SERVER_PID" > /dev/null 2>&1 || true
    fi
    rm -rf "$WORK_DIR"
    echo -e "${GREEN}===> [Frontend E2E Cleanup] Cleanup complete.${NC}"
}
trap cleanup EXIT INT TERM

# 1. Build Binaries
echo -e "\n${YELLOW}[Step 1/5] Compiling binaries (app, web)...${NC}"
go build -o "$WORK_DIR/app" ./cmd/app
go build -o "$WORK_DIR/web" ./cmd/web
echo -e "${GREEN}Binaries successfully compiled.${NC}"

# 2. Start Core Server (SQLite Standalone)
echo -e "\n${YELLOW}[Step 2/5] Starting Core Server (SQLite) on port ${SERVER_PORT}...${NC}"
PORT="$SERVER_PORT" \
PPROF_PORT="16060" \
DATABASE_DRIVER="sqlite3" \
DATABASE_DSN="file:$WORK_DIR/template.db?cache=shared&mode=rwc&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" \
BACKUP_DIR="$BACKUP_DIR" \
BACKUP_ENABLED="false" \
    "$WORK_DIR/app" server --port "$SERVER_PORT" > "$WORK_DIR/server.log" 2>&1 &
SERVER_PID=$!

# 3. Wait for Core Server Health & Seed Initial Data
echo -e "\n${YELLOW}[Step 3/5] Waiting for Core Server to become healthy and seeding initial user...${NC}"
for i in {1..30}; do
    if curl -sf "http://127.0.0.1:${SERVER_PORT}/v1/system/healthz" > /dev/null 2>&1; then
        echo -e "${GREEN}Core Server is ONLINE!${NC}"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo -e "${RED}Core Server failed to start in time. Logs:${NC}"
        cat "$WORK_DIR/server.log"
        exit 1
    fi
    sleep 1
done

echo "  - Provisioning initial admin user..."
curl -sf -X POST "http://127.0.0.1:${SERVER_PORT}/v1/users" \
  -H "Content-Type: application/json" \
  -d '{"name": "admin", "email": "admin@example.com"}' > /dev/null || {
    echo -e "${YELLOW}User already exists or created via migration${NC}"
}

# 4. Start Web Frontend Server
echo -e "\n${YELLOW}[Step 4/5] Starting Web Frontend Server on port ${WEB_PORT}...${NC}"
PORT="$WEB_PORT" \
DATABASE_DRIVER="sqlite3" \
DATABASE_DSN="file:$WORK_DIR/template.db?cache=shared&mode=rwc&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" \
BACKUP_DIR="$BACKUP_DIR" \
    "$WORK_DIR/web" --port "$WEB_PORT" > "$WORK_DIR/web.log" 2>&1 &
WEB_PID=$!

for i in {1..20}; do
    if curl -sf "http://127.0.0.1:${WEB_PORT}/healthz" > /dev/null 2>&1; then
        echo -e "${GREEN}Web Frontend Server is ONLINE!${NC}"
        break
    fi
    if [ "$i" -eq 20 ]; then
        echo -e "${RED}Web frontend failed to start in time. Logs:${NC}"
        cat "$WORK_DIR/web.log"
        exit 1
    fi
    sleep 0.5
done

# 5. Execute Python Frontend UI & Snapshot Verification
echo -e "\n${YELLOW}[Step 5/5] Running Headless Chrome E2E Verification & Snapshot Suite...${NC}"
WEB_URL="http://127.0.0.1:${WEB_PORT}" CORE_URL="http://127.0.0.1:${SERVER_PORT}" \
    python3 scripts/test_frontend_ui.py

echo -e "\n${GREEN}========================================================================${NC}"
echo -e "${GREEN} ✅ ALL FRONTEND (NO-DOCKER) E2E TESTS PASSED SUCCESSFULLY!             ${NC}"
echo -e "${GREEN}    - Standalone Web Server: Running without Docker                     ${NC}"
echo -e "${GREEN}    - HTMX System Metric Cards: Process CPU, Memory, Goroutines OK      ${NC}"
echo -e "${GREEN}    - User Directory Table: 100% Validated                              ${NC}"
echo -e "${GREEN}    - Backup Lifecycle: Real-time trigger & tar.gz swap verified        ${NC}"
echo -e "${GREEN}    - Snapshot Generated: docs/images/frontend_dashboard.png            ${NC}"
echo -e "${GREEN}    - HTML Test Report: test_reports/frontend_e2e_report.html           ${NC}"
echo -e "${GREEN}========================================================================${NC}"
