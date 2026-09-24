# さくらのクラウド オブジェクトストレージ × OpenTelemetry Collector 実践リポジトリ

このリポジトリは、**さくらのクラウド オブジェクトストレージ（S3互換・東京第1サイト `tky01` / 石狩サイト `isk01`）** に対して、**OpenTelemetry Collector (`sacloud-otel-collector`)** を用いて構造化ログ（OTLP gRPC/HTTP）および Docker コンテナログを効率的に集約・保管・検証するための実践リファレンス実装です。

また、エンタープライズ Go 開発用テンプレート（**Node.js不要のスタンドアロン HTMX フロントエンド**、**改変検知付き SQLite バックアップ＆アトミックリストア**、**多層 E2E テストフレームワーク**、**AI カスタムスキル**）も完全同梱しています。

---

## 🚀 主な特徴

1. **さくらのクラウド オブジェクトストレージ × `awss3` exporter 連携**:
   - `sacloud-otel-collector` の `awss3` exporter を使用し、OTLP ログを gzip 圧縮してオブジェクトストレージへ直接ストリーミング転送。
   - S3 互換ストレージ向けにパススタイルアクセス（`s3_force_path_style: true`）を標準構成。
2. **Docker コンテナログ収集パイプライン (`filelog` receiver)**:
   - ホスト上の Docker JSON ログ（`/var/lib/docker/containers/*/*-json.log`）を自動検知・ tail 収集。
   - JSON ログからログ本文（`message` / `body`）を抽出し、コンテナ名やカスタム属性（`run_id` 等）を属性マップへ構造化。
3. **AWS SDK v2 チェックサムヘッダー互換性対策**:
   - S3 互換ストレージ（さくらのオブジェクトストレージや MinIO）で発生する `400 InvalidDigest` / `NotImplemented` エラーを防止するため、チェックサム計算制御フラグ（`AWS_RESPONSE_CHECKSUM_VALIDATION=WHEN_REQUIRED`, `AWS_REQUEST_CHECKSUM_CALCULATION=WHEN_REQUIRED`）を自動注入。
4. **JST (日本時間) タイムゾーン対応パーティショニング**:
   - Dockerfile での `tzdata` 導入と `TZ=Asia/Tokyo` 設定により、オブジェクトストレージ上の日付・時間階層（`logs/%Y/%m/%d/%H/`）が JST 基準で正しく作成されます。
5. **純Go製 E2E 検証ツール (`sacloud-otel-verifier`)**:
   - TraceID / SpanID / RunID を付与した OTLP ログ送出。
   - ストレージから最新オブジェクトを自動取得し、gzip 解凍、OTLP JSON / Docker JSON ログのパース、ID 突合アサーションを実行。
6. **1 コマンド全自動検証**:
   - `make verify-sakura`（ホスト実行）および `make verify-docker-log-sakura`（Docker Compose 実行）により、Collector の起動、ログ送出、S3 突合アサーション、終了クリーンアップまでを全自動実行。
7. **スタンドアロン HTMX フロントエンド & SSG (Node.js/npm 完全不要)**:
   - `//go:embed` によるローカル内包、リアルタイムメトリクス更新、静的サイト事前レンダリング出力（SSG）。
8. **改変検知付き SQLite バックアップ＆アトミックリストア**:
   - SHA-256 チェックサム付きマニフェストによるバックアップアーカイブ（`tar.gz`）とトランザクション復元。

---

## 🏗️ アーキテクチャ概要

```mermaid
graph TD
    subgraph "Log Sources"
        App["App / Services (OTLP gRPC 4317)"]
        DockerLog["Docker Engine (/var/lib/docker/containers/*-json.log)"]
        VerifierEmitter["sacloud-otel-verifier emit"]
    end

    subgraph "OpenTelemetry Collector Layer (sacloud-otel-collector)"
        Receiver["Receivers:<br/>• otlp (gRPC :4317 / HTTP :4318)<br/>• filelog (Docker JSON Logs)"]
        Processor["Processors:<br/>• batch (1-2s)<br/>• memory_limiter"]
        Exporter["Exporter: awss3<br/>• gzip compression<br/>• s3_force_path_style: true<br/>• partition: logs/%Y/%m/%d/%H/"]
    end

    subgraph "Object Storage (S3-Compatible)"
        SakuraS3["さくらのクラウド オブジェクトストレージ<br/>(tky01 / isk01 Site)"]
        LocalStack["LocalStack / MinIO<br/>(Local Dev & CI)"]
    end

    subgraph "Verification Layer (sacloud-otel-verifier)"
        VerifierInspect["sacloud-otel-verifier verify<br/>1. S3 ListObjectsV2 & GetObject<br/>2. gzip Decompress<br/>3. Parse OTLP / Docker JSON<br/>4. Assert RunID / TraceID Match"]
    end

    App -->|OTLP gRPC| Receiver
    VerifierEmitter -->|OTLP gRPC| Receiver
    DockerLog -->|Tail Read & JSON Parse| Receiver
    Receiver --> Processor --> Exporter
    Exporter -->|PUT Object (JST Partition)| SakuraS3
    Exporter -.->|Local Testing| LocalStack
    VerifierInspect -->|Assert Delivery| SakuraS3
    VerifierInspect -.->|Assert Local| LocalStack
```

---

## 🛠️ クイックスタート

### 1. 環境変数の設定 (`.env`)

`.env.example` をコピーして `.env` を作成し、さくらのクラウド オブジェクトストレージの認証情報を入力します：

```bash
cp .env.example .env
```

```ini
# .env
SAKURA_ACCESS_KEY_ID=your_access_key_here
SAKURA_SECRET_ACCESS_KEY=your_secret_key_here
SAKURA_BUCKET_NAME=your-bucket-name
SAKURA_REGION=jp-north-2
SAKURA_ENDPOINT=https://s3.isk01.sakurastorage.jp
# ※ 東京第1サイトの場合は https://s3.tky01.sakurastorage.jp
```

> [!IMPORTANT]
> `.env` には実際のクレデンシャルが保存されます。`.gitignore` に登録されているため Git にコミットされることはありませんが、ファイルの取り扱いには十分ご注意ください。

### 2. さくらのクラウド オブジェクトストレージ検証の実行

#### ① ホスト上での OTLP ログ転送・突合検証（約 5 秒で完了）
```bash
make verify-sakura
```
Collector がバックグラウンドで自動起動し、OTLP ログを送出、オブジェクトストレージから取得して検証（`Overall Status: ✅ PASSED`）後、プロセスを自動クリーンアップします。

#### ② Docker コンテナログ収集＆転送検証（約 15 秒で完了）
```bash
make verify-docker-log-sakura
```
Docker Compose で Collector とログ生成コンテナを起動し、Docker ログが `filelog` receiver 経由で取得され、JST パーティションに到達することを検証します。

#### ③ 完全ローカル環境（LocalStack）での検証（外部通信不要）
```bash
make verify-otel-local
make verify-docker-log-local
```

---

## ⚙️ コマンド一覧

### オブジェクトストレージ & OpenTelemetry 検証
| コマンド | 説明 |
| :--- | :--- |
| `make verify-sakura` | **★推奨** さくらのクラウド連携 1 コマンド全自動検証（ホスト Collector 自動起動・検証・終了） |
| `make verify-docker-log-sakura` | Docker コンテナログ収集＆さくらオブジェクトストレージ転送全自動検証 |
| `make verify-otel-local` | LocalStack を用いた完全ローカル OTLP ログ転送 E2E 検証 |
| `make verify-docker-log-local` | LocalStack を用いた完全ローカル Docker ログ収集 E2E 検証 |
| `make build-verifier` | 純Go製検証ツール `bin/sacloud-otel-verifier` のビルド |

### アプリケーション & テンプレート機能
| コマンド | 説明 |
| :--- | :--- |
| `make run` | スタンドアロンサーバー（Core API + Web UI）のローカル一括起動 |
| `make test` | データ競合検知 (`-race`) およびカバレッジ測定付き単体テスト |
| `make sqlite-e2e` | Docker 不要の超高速 SQLite E2E テストの実行 |
| `make frontend-e2e` | スタンドアロン HTMX フロントエンド E2E テスト & スナップショット生成 |
| `make docker-e2e` | Docker Compose フルスタック E2E テスト & Grafana 検証 |
| `make ssg-build` | Go テンプレートからの静的サイト事前レンダリング出力 (SSG) |
| `make fmt` | ソースコードのフォーマットおよびリンターによる自動修正 |
| `make lint` | `golangci-lint` を使用した静的解析の実行 |
| `make vulncheck` | `govulncheck` を使用した脆弱性診断の実行 |
| `make license-check` | Go ソースコードのライセンス＆作成者ヘッダーの検証 |
| `make self-eval` | リポジトリ要件の自己評価の実行 (`REQUIREMENTS.md` の更新) |

---

## 📚 ドキュメント一覧

- 📖 **[さくらのクラウド オブジェクトストレージ 連携ガイド (docs/sacloud_otel_s3_guide.md)](docs/sacloud_otel_s3_guide.md)**:
  - アーキテクチャ詳細、Collector 設定（YAML）仕様、AWS SDK v2 チェックサム問題の技術解説、トラブルシューティング、実機動作エビデンス集。
- 🏛️ **[システムアーキテクチャ詳細 (docs/architecture.md)](docs/architecture.md)**:
  - コンポーネント設計、データフロー、多層 E2E テスト構成。
- 📋 **[要件チェックリスト & 自己評価 (REQUIREMENTS.md)](REQUIREMENTS.md)**:
  - 全要件の遵守状況および `make self-eval` による 100% 準拠スコア。
- 🚀 **[デプロイ＆他リポジトリ展開ガイド (docs/deployment_guide_and_templates.md)](docs/deployment_guide_and_templates.md)**:
  - カスタムスキルの横展開手順とトラブルシューティング。

---

## 🔒 セキュリティとクレデンシャル管理

本リポジトリでは、機密情報の漏洩を防ぐため以下のルールを徹底しています：
- `.env`, `.env.*`, `*.resolved.yaml` などの実値設定ファイルは `.gitignore` に明示的に除外されています。
- `main.go` では機密情報のログマスキングを実装しています。
- CI では静的解析（`make lint`）、脆弱性診断（`make vulncheck`）、ライセンスヘッダー検査（`make license-check`）を全自動実行しています。

