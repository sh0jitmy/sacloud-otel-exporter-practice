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

echo -e "${CYAN}====================================================${NC}"
echo -e "${CYAN}        Go Template Full-Stack Live Demo Runner     ${NC}"
echo -e "${CYAN}====================================================${NC}"

# 1. Start Docker Compose Stack
echo -e "\n${YELLOW}[Step 1/4] Building & Starting Docker Compose Stack...${NC}"
docker compose up -d --build

# 2. Wait for Health Checks
echo -e "\n${YELLOW}[Step 2/4] Waiting for services to become HEALTHY...${NC}"

echo -n "Waiting for Core API Server (:8080)..."
until curl -sf http://localhost:8080/v1/system/healthz > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo -e " ${GREEN}[ONLINE]${NC}"

echo -n "Waiting for Web Dashboard (:3001)..."
until curl -sf http://localhost:3001/healthz > /dev/null 2>&1; do
    echo -n "."
    sleep 1
done
echo -e " ${GREEN}[ONLINE]${NC}"

echo -n "Waiting for VictoriaMetrics (:8428)..."
until curl -sf http://localhost:8428/health > /dev/null 2>&1; do
    echo -n "."
    sleep 1
done
echo -e " ${GREEN}[ONLINE]${NC}"

echo -n "Waiting for Grafana (:3000)..."
until curl -sf http://localhost:3000/api/health > /dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo -e " ${GREEN}[ONLINE]${NC}"

# 3. Seed Initial Demo Data
echo -e "\n${YELLOW}[Step 3/4] Seeding Demo User Data & Initial Backup Archive...${NC}"
curl -s -X POST http://localhost:8080/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name": "demo-operator", "email": "operator@demo.internal"}' > /dev/null || true

curl -s -X POST http://localhost:8080/v1/system/backups > /dev/null || true
echo -e "  ${GREEN}Initial demo records & snapshot created.${NC}"

# 4. Summary & Endpoints
echo -e "\n${GREEN}====================================================${NC}"
echo -e "${GREEN}     🎉 Go Template Demo Stack is LIVE & READY      ${NC}"
echo -e "${GREEN}====================================================${NC}"
echo -e "🔹 ${CYAN}HTMX Web Dashboard${NC}   : http://localhost:3001"
echo -e "🔹 ${CYAN}REST API / Health${NC}    : http://localhost:8080/v1/system/healthz"
echo -e "🔹 ${CYAN}OpenAPI Specification${NC}: http://localhost:8080/openapi.yaml"
echo -e "🔹 ${CYAN}Grafana Dashboard${NC}    : http://localhost:3000/d/template-overview"
echo -e "🔹 ${CYAN}VictoriaMetrics TSDB${NC} : http://localhost:8428"
echo -e "🔹 ${CYAN}PostgreSQL Database${NC}  : postgresql://template:secret@localhost:5432/templatedb"
echo -e "\nTo stop the demo: ${YELLOW}docker compose down${NC}"
