# CLAUDE.md

## プロジェクトの概要

このプロジェクトは、**さくらのクラウド オブジェクトストレージ（S3互換）× OpenTelemetry Collector (`sacloud-otel-collector`)** による構造化ログおよび Docker コンテナログの集約・検証リポジトリです。
また、Go開発向けのプロダクション品質テンプレート（HTMX フロントエンド、改変検知 SQLite ガバナンス、多層 E2E テスト）および**日本語カスタムスキル（`.claude/skills/`）**が同梱されています。

## AIエージェント（Claude Code）への指示

> [!IMPORTANT]
> - **スキルの厳格な適用**: 本プロジェクトにおけるコードの実装、設計、リファクタリング、およびコードレビューを行う際は、必ず `.claude/skills/` にある各スキル（`golang-implementation`、`golang-htmx-frontend`、`golang-sqlite-governance`、`multi-tier-e2e-testing`、`observability-stack` 等）の指針に準拠してください。
> - **言語の統一**: コミットメッセージは**英語**、それ以外のPR説明、Issue、およびAIによるレビューレポートは**完全な日本語**で記述してください。
> - **ライセンスヘッダーの維持**: 新規追加した Go ソースコードには必ず Apache-2.0 ライセンスヘッダーを付与し、`make license-check` をパスさせてください。

## 開発・検証コマンド一覧

### さくらのクラウド オブジェクトストレージ & OTel 検証
- **さくらのクラウド 1 コマンド全自動検証**: `make verify-sakura`
- **Docker コンテナログ収集＆さくら転送検証**: `make verify-docker-log-sakura`
- **LocalStack ローカル OTLP 検証**: `make verify-otel-local`
- **LocalStack ローカル Docker ログ検証**: `make verify-docker-log-local`

### Go開発 & ローカル実行
- **ローカル一括起動 (API + Web)**: `make run`
- **コードフォーマット**: `make fmt`
- **静的解析の実行**: `make lint`
- **脆弱性スキャンの実行**: `make vulncheck`
- **単体テストの実行**: `make test`
- **マルチバイナリビルド**: `make build`
- **静的サイト生成 (SSG)**: `make ssg-build`

### 多層 E2E テスト
- **No-Docker SQLite E2E**: `make sqlite-e2e`
- **スタンドアロン HTMX フロントエンド E2E**: `make frontend-e2e`
- **Docker Compose フルスタック E2E**: `make docker-e2e`
- **フルスタックライブデモ**: `make demo`

### 品質・スキル管理タスク
- **カスタムスキルのインストール**: `make install-all` (Claude & Antigravity)
- **プロジェクトの要件セルフチェック**: `make self-eval`
- **スキルの構文検証**: `make check`

