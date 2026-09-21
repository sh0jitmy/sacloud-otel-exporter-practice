#!/usr/bin/env bash
# Copyright 2026 [Copyright Holder]
# Licensed under the Apache License, Version 2.0 (the "License");

set -euo pipefail

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${CYAN}================================================================${NC}"
echo -e "${CYAN}   Go Template Standalone SQLite (No-Docker) E2E Test Suite     ${NC}"
echo -e "${CYAN}================================================================${NC}"

WORK_DIR=$(mktemp -d -t template-sqlite-e2e-XXXXXX)
BACKUP_DIR="$WORK_DIR/backups"
mkdir -p "$BACKUP_DIR"

SERVER_PORT=18080
SERVER_PID=""

cleanup() {
    echo -e "\n${YELLOW}===> [E2E Cleanup] Stopping test server and removing temp files...${NC}"
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" > /dev/null 2>&1; then
        kill "$SERVER_PID" > /dev/null 2>&1 || true
    fi
    rm -rf "$WORK_DIR"
    echo -e "${GREEN}===> [E2E Cleanup] Cleanup complete.${NC}"
}
trap cleanup EXIT INT TERM

# 1. Compile binary
echo -e "\n${YELLOW}[Step 1/6] Compiling binary (bin/app)...${NC}"
go build -o "$WORK_DIR/app" ./cmd/app
echo -e "${GREEN}Binary compiled successfully.${NC}"

# 2. Start server
echo -e "\n${YELLOW}[Step 2/6] Starting Standalone API Server on port ${SERVER_PORT}...${NC}"
PORT="$SERVER_PORT" \
DATABASE_DRIVER="sqlite3" \
DATABASE_DSN="file:$WORK_DIR/app.db?cache=shared&mode=rwc&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" \
BACKUP_DIR="$BACKUP_DIR" \
    "$WORK_DIR/app" server --port "$SERVER_PORT" > "$WORK_DIR/server.log" 2>&1 &
SERVER_PID=$!

# 3. Wait for Healthz and Readyz
echo -e "\n${YELLOW}[Step 3/6] Waiting for server health and readiness probes...${NC}"
for i in {1..30}; do
    if curl -sf "http://127.0.0.1:${SERVER_PORT}/v1/system/healthz" > /dev/null 2>&1; then
        echo -e "${GREEN}Server is ONLINE and HEALTHY!${NC}"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo -e "${RED}Server failed to start in time. Logs:${NC}"
        cat "$WORK_DIR/server.log"
        exit 1
    fi
    sleep 0.5
done

curl -sf "http://127.0.0.1:${SERVER_PORT}/v1/system/readyz" > /dev/null
echo -e "${GREEN}Readiness probe /v1/system/readyz verified (READY).${NC}"

# 4. Authentication Flow (Login & /users/me)
echo -e "\n${YELLOW}[Step 4/6] Testing Authentication & Bearer Token Flow...${NC}"
LOGIN_RESP=$(curl -sf -X POST "http://127.0.0.1:${SERVER_PORT}/v1/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin-pass"}')
TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
    echo -e "${RED}Failed to extract JWT token from login response!${NC}"
    exit 1
fi
echo "  - Successfully logged in as admin. Token: ${TOKEN:0:15}..."

# Access protected /v1/users/me
USER_RESP=$(curl -sf -X GET "http://127.0.0.1:${SERVER_PORT}/v1/users/me" \
  -H "Authorization: Bearer ${TOKEN}")
echo "  - User profile verified: $USER_RESP"

# 5. Backup Creation, Download & Verification
echo -e "\n${YELLOW}[Step 5/6] Testing Full SQLite Backup Creation & Archive Download...${NC}"
BACKUP_RESP=$(curl -sf -X POST "http://127.0.0.1:${SERVER_PORT}/v1/system/backups")
echo "  - Backup API response: $BACKUP_RESP"
BACKUP_FILE=$(echo "$BACKUP_RESP" | grep -o '"filename":"[^"]*' | cut -d'"' -f4)
DOWNLOAD_URL=$(echo "$BACKUP_RESP" | grep -o '"download_url":"[^"]*' | cut -d'"' -f4)

DOWNLOAD_PATH="$WORK_DIR/downloaded_backup.tar.gz"
curl -sf "http://127.0.0.1:${SERVER_PORT}${DOWNLOAD_URL}" -o "$DOWNLOAD_PATH"

echo "  - Validating archive contents:"
tar -tzf "$DOWNLOAD_PATH" | sed 's/^/    /'

# 6. Database Restore & Retention Purge
echo -e "\n${YELLOW}[Step 6/6] Testing Database Restore & Retention Purge...${NC}"
RESTORE_RESP=$(curl -sf -X POST "http://127.0.0.1:${SERVER_PORT}/v1/system/restores" \
  -H "Content-Type: application/json" \
  -d "{\"archive_path\": \"${BACKUP_FILE}\"}")
echo "  - Restore response: $RESTORE_RESP"

# Verify admin is still accessible
curl -sf -X GET "http://127.0.0.1:${SERVER_PORT}/v1/users/me" \
  -H "Authorization: Bearer ${TOKEN}" > /dev/null
echo "  - Verified authenticated access intact after restore"

# Test Purge
PURGE_RESP=$(curl -sf -X POST "http://127.0.0.1:${SERVER_PORT}/v1/system/purge" \
  -H "Content-Type: application/json" \
  -d '{"retention_days": 30}')
echo "  - Purge response: $PURGE_RESP"

echo -e "\n${GREEN}========================================================================${NC}"
echo -e "${GREEN} ✅ ALL SQLITE (NO-DOCKER) E2E TESTS PASSED SUCCESSFULLY!                ${NC}"
echo -e "${GREEN}    - Standalone SQLite DB: Operational without Docker                  ${NC}"
echo -e "${GREEN}    - Healthz & Readyz Probes: Verified                                 ${NC}"
echo -e "${GREEN}    - Authentication & Protected API: Verified                          ${NC}"
echo -e "${GREEN}    - Backup & Transactional Restore: Verified                          ${NC}"
echo -e "${GREEN}    - Data Retention Purge: Verified                                    ${NC}"
echo -e "${GREEN}========================================================================${NC}"
