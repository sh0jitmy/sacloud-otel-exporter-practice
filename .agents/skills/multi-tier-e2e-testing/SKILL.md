---
name: multi-tier-e2e-testing
description: "単体テスト、No-Docker SQLite E2E、HTMXフロントエンドE2E（Headless Chrome / HTMLレポート）、Docker Compose E2E、およびGrafana UI検証から構成される多層E2Eテストフレームワークの設計と実行プラクティス。"
user-invocable: true
license: Apache-2.0
compatibility: Designed for Claude Code, Cursor, Antigravity, OpenCode, OpenClaw, and other AI coding agents.
metadata:
  author: [YOUR_NAME]
  version: "1.0.0"
  openclaw:
    emoji: "🔬"
    homepage: https://github.com/shjtmy/go_sh0jitmy_template
    requires:
      bins:
        - go
        - python3
    install: []
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Bash(python3:*) Bash(scripts/*) Agent AskUserQuestion
---

# Multi-Tier E2E Testing Framework

## 1. テスト階層ピラミッド
```
+-------------------------------------------------------------+
| Layer 4: Docker Full-Stack & Grafana E2E (make docker-e2e)  | -> PostgreSQL, VictoriaMetrics, Grafana, API, Web
+-------------------------------------------------------------+
| Layer 3: Standalone Frontend E2E (make frontend-e2e)        | -> Headless Chrome, HTMX Swaps, HTML Reports
+-------------------------------------------------------------+
| Layer 2: Standalone SQLite E2E (make sqlite-e2e)            | -> No-Docker, Zero External Dep, < 5s Fast E2E
+-------------------------------------------------------------+
| Layer 1: Unit & Integration Tests (make test)               | -> t.Parallel(), goleak, In-memory SQLite
+-------------------------------------------------------------+
```

## 2. 各レイヤーの責務と実行方法
1. **Layer 1: Unit & Integration (`make test`)**:
   - カバレッジ 80% 以上を維持。
   - `file:<test_name>?mode=memory&cache=shared` によるインメモリDB完全分離。
2. **Layer 2: Standalone SQLite E2E (`make sqlite-e2e`)**:
   - `scripts/sqlite_e2e.sh`
   - Docker 不要、外部サービス不要。
   - 認証フロー、CRUD、バックアップ作成、整合性検証、リストア、保持期間パージを一括検証。
3. **Layer 3: Standalone Frontend E2E (`make frontend-e2e`)**:
   - `scripts/frontend_e2e.sh` & `scripts/test_frontend_ui.py`
   - Headless Chrome による自動スナップショット取得 (`docs/images/frontend_dashboard.png`)。
   - HTMX によるシステムメトリクス取得、テーブル表示、バックアップ作成アクションを網羅。
   - スタンドアロン HTML レポート生成 (`test_reports/frontend_e2e_report.html`)。
4. **Layer 4: Docker Full-Stack E2E (`make docker-e2e`)**:
   - `scripts/docker_e2e.sh` & `scripts/test_grafana_ui.py`
   - PostgreSQL、VictoriaMetrics、Grafana、コアサーバー、Webサーバーのコンテナ間連携を検証。
   - Grafana UI スナップショットとメトリクス検証レポート生成。
