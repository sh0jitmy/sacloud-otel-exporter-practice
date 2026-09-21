// Copyright 2026 [Copyright Holder]
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: [YOUR_NAME]

# ADR 0004: 多層 E2E テストフレームワーク (Multi-Tier E2E Testing Framework)

## ステータス
承認済み (Accepted)

## コンテキスト
システムの本番品質を保証するためには、単体テスト（Unit Test）だけでなく、実際のHTTPリクエスト、データベース永続化、バックアップ・リストア、フロントエンドUIレンダリング、可観測性（Grafana/VictoriaMetrics）メトリクスの整合性までを網羅する包括的な検証が必要です。
しかし、全てのテストで Docker を必須とすると、開発サイクルの遅延（フィードバックループの鈍化）やリソース制限のあるCI環境での失敗を招きます。

## 意思決定

以下の 4 層からなる「多層 E2E テストフレームワーク」を構築し、実行速度と検証深度の両立を図ります。

1. **Layer 1: 単体 & 結合テスト (`make test`)**:
   - `t.Parallel()` による高速並行実行。
   - テスト毎に分離されたインメモリ SQLite DSN（`file:<test_name>?mode=memory&cache=shared`）を用いてデータの競合を防止。
   - `go.uber.org/goleak` による Goroutine リーク検証。
   - カバレッジ 80% 以上を必須化。
2. **Layer 2: Standalone SQLite E2E (`make sqlite-e2e`)**:
   - `scripts/sqlite_e2e.sh`
   - Docker 不要、外部依存ゼロ。約 2〜3 秒で実行完了。
   - サーバープロセス起動、ヘルスチェック/Readinessプローブ、JWT/Bearer認証、CRUD、バックアップ作成、整合性検証、リストア、保持期間パージを一括検証。
3. **Layer 3: Standalone HTMX Frontend E2E (`make frontend-e2e`)**:
   - `scripts/frontend_e2e.sh` & `scripts/test_frontend_ui.py`
   - Headless Chrome を用いた自動スナップショット撮影（`docs/images/frontend_dashboard.png`）。
   - HTMX によるシステムリソースメトリクス取得、ユーザー一覧テーブル、リアルタイムバックアップ作成アクションを検証。
   - スタンドアロン HTML レポート生成（`test_reports/frontend_e2e_report.html`）。
4. **Layer 4: Docker Full-Stack & Grafana E2E (`make docker-e2e`)**:
   - `scripts/docker_e2e.sh` & `scripts/test_grafana_ui.py`
   - PostgreSQL、VictoriaMetrics、Grafana、コアAPIサーバー、Webサーバーのマルチコンテナ協調動作を検証。
   - Grafana API ヘルスチェック、ダッシュボードメタデータ、VictoriaMetrics プロメテウスメトリクス（Goroutine 数、ヒープメモリ等）の妥当性を自動アサーション。
   - Grafana UI スナップショット（`docs/images/grafana_screenshot.png`）および HTML レポート生成。

## 帰結

- **利点**:
  - ローカル開発時は `make sqlite-e2e` や `make frontend-e2e` により、Docker を立ち上げることなく瞬時に完全な E2E 検証が可能。
  - CI 環境でも `sqlite-e2e` と `frontend-e2e` が迅速に実行され、HTML レポートとスナップショットがアーティファクトとして自動保存される。
  - リリース前や統合環境では `make docker-e2e` により、本番同様のコンテナスタックと可観測性を確実に保証。
