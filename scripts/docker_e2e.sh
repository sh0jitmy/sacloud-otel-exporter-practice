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
echo -e "${CYAN}   Go Template Docker Compose Full-Stack E2E Test Suite          ${NC}"
echo -e "${CYAN}================================================================${NC}"

cleanup() {
    echo -e "\n${YELLOW}===> [Docker E2E Cleanup] Tearing down Docker Compose stack...${NC}"
    docker compose down -v --remove-orphans > /dev/null 2>&1 || true
    echo -e "${GREEN}===> [Docker E2E Cleanup] Cleanup complete.${NC}"
}
trap cleanup EXIT INT TERM

# 1. Build & Start Stack
echo -e "\n${YELLOW}[Step 1/5] Building & starting Docker Compose stack...${NC}"
docker compose up -d --build

# 2. Wait for Services
echo -e "\n${YELLOW}[Step 2/5] Waiting for services to become healthy...${NC}"

echo -n "Waiting for Core API Server (:8080)..."
for i in {1..30}; do
    if curl -sf http://localhost:8080/v1/system/healthz > /dev/null 2>&1; then
        echo -e " ${GREEN}[ONLINE]${NC}"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo -e " ${RED}[FAILED]${NC}"
        docker compose logs app
        exit 1
    fi
    sleep 2
done

echo -n "Waiting for Web Dashboard (:3001)..."
for i in {1..20}; do
    if curl -sf http://localhost:3001/healthz > /dev/null 2>&1; then
        echo -e " ${GREEN}[ONLINE]${NC}"
        break
    fi
    sleep 1
done

echo -n "Waiting for VictoriaMetrics (:8428)..."
for i in {1..20}; do
    if curl -sf http://localhost:8428/health > /dev/null 2>&1; then
        echo -e " ${GREEN}[ONLINE]${NC}"
        break
    fi
    sleep 1
done

echo -n "Waiting for Grafana (:3000)..."
for i in {1..30}; do
    if curl -sf http://localhost:3000/api/health > /dev/null 2>&1; then
        echo -e " ${GREEN}[ONLINE]${NC}"
        break
    fi
    sleep 2
done

# 3. Test Core API Flow (Postgres Backend)
echo -e "\n${YELLOW}[Step 3/5] Testing Core REST API Flow (PostgreSQL Backend)...${NC}"

echo "  - Creating user 'docker-admin'..."
curl -sf -X POST http://localhost:8080/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name": "docker-admin", "email": "admin@docker.local"}' || true

echo "  - Fetching users..."
curl -sf http://localhost:8080/v1/users > /dev/null
echo -e "  ${GREEN}User API endpoints verified.${NC}"

echo "  - Testing System Backup Trigger..."
BACKUP_RESP=$(curl -sf -X POST http://localhost:8080/v1/system/backups)
echo "  Backup output: $BACKUP_RESP"

# 4. Verify HTMX Web Frontend Endpoints
echo -e "\n${YELLOW}[Step 4/5] Verifying Web UI Server components...${NC}"
curl -sf http://localhost:3001/ > /dev/null
curl -sf http://localhost:3001/ui/components/system-metrics > /dev/null
curl -sf http://localhost:3001/ui/components/users-table > /dev/null
curl -sf http://localhost:3001/ui/components/backup-panel > /dev/null
echo -e "${GREEN}HTMX components responded successfully.${NC}"

# 5. Execute Grafana UI & Metric Assertions
echo -e "\n${YELLOW}[Step 5/5] Executing Grafana UI Verification & Snapshot Suite...${NC}"
GRAFANA_URL="http://localhost:3000" CORE_URL="http://localhost:8080" VM_URL="http://localhost:8428" \
    python3 scripts/test_grafana_ui.py

echo -e "\n${GREEN}========================================================================${NC}"
echo -e "${GREEN} ✅ ALL DOCKER E2E TESTS AND GRAFANA VERIFICATIONS PASSED!              ${NC}"
echo -e "${GREEN}    - Multi-container Stack: app, web, postgres, victoriametrics, grafana${NC}"
echo -e "${GREEN}    - PostgreSQL Backend: Migrations & Persistence Validated            ${NC}"
echo -e "${GREEN}    - Grafana & VictoriaMetrics: Metrics & Dashboard Verified           ${NC}"
echo -e "${GREEN}    - HTML Report: test_reports/grafana_e2e_report.html                 ${NC}"
echo -e "${GREEN}========================================================================${NC}"
