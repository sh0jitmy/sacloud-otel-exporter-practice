# sacloud-otel-collector & s3exporter によるさくらのオブジェクトストレージ連携 完全検証ガイド

本ドキュメントでは、さくらインターネット公式の [sacloud/sacloud-otel-collector](https://github.com/sacloud/sacloud-otel-collector)（カスタム OpenTelemetry Collector）に標準内包されている `awss3` エクスポーター（`awss3exporter`）を活用し、アプリケーションから OpenTelemetry (OTel) 経由で出力した構造化ログを「さくらのオブジェクトストレージ」へ書き出し、内容を自動検証する手順を解説します。

---

## 1. アーキテクチャ構成

```
+-----------------------------------------------------------------------------------+
| 検証ソフトウェア (sacloud-otel-verifier / cmd/verifier)                              |
|                                                                                   |
|  [Log Emitter (OTel Go SDK + otelslog)]                                           |
|   1. ユニークな test_run_id（UUID）と分散トレースContext（TraceID, SpanID）を生成    |
|   2. 構造化ログ（slog.InfoContext 等）を OTel LoggerProvider 経由で送出               |
|   3. OTLP gRPC (ポート 4317) または HTTP (ポート 4318) で Collector へ即時 Flush 送信  |
+-----------------------------------------------------------------------------------+
                                          |
                                          | OTLP (gRPC :4317 / HTTP :4318)
                                          v
+-----------------------------------------------------------------------------------+
| sacloud-otel-collector (v0.8.0+ / 公式バイナリまたはコンテナ)                        |
|                                                                                   |
|  - Receivers:  otlp (0.0.0.0:4317 / 4318)                                         |
|  - Processors: memory_limiter, batch (バッチ集約)                                  |
|  - Exporters:  awss3 (内包 exporter)                                              |
|      s3uploader:                                                                  |
|        endpoint: https://s3.isk01.sakurastorage.jp                                |
|        region: jp-north-1                                                         |
|        s3_bucket: ${SACLOUD_O2_BUCKET}                                            |
|        s3_prefix: logs                                                            |
|        s3_partition_format: '%Y/%m/%d/%H'                                         |
|        s3_force_path_style: true                                                  |
|      compression: gzip                                                            |
|      marshaler: otlp_json                                                         |
|                                                                                   |
|  - 必須環境変数 (互換フラグ):                                                      |
|      AWS_RESPONSE_CHECKSUM_VALIDATION=WHEN_REQUIRED                               |
|      AWS_REQUEST_CHECKSUM_CALCULATION=WHEN_REQUIRED                               |
+-----------------------------------------------------------------------------------+
                                          |
                                          | S3 API (PUT Object: logs/YYYY/MM/DD/HH/*.json.gz)
                                          v
+-----------------------------------------------------------------------------------+
| さくらのオブジェクトストレージ (S3互換ストレージ)                                   |
| バケット内に OTLP JSON (gzip圧縮) ログアーカイブが永続化される                         |
+-----------------------------------------------------------------------------------+
                                          ^
                                          | S3 API (ListObjectsV2, GetObject)
+-----------------------------------------+-----------------------------------------+
| 検証ソフトウェア (sacloud-otel-verifier / cmd/verifier)                              |
|                                                                                   |
|  [S3 Log Verifier]                                                                |
|   1. さくらのオブジェクトストレージから対象プレフィックスの最新オブジェクトを取得 |
|   2. gzip 解凍を行い、OTLP JSON 構造をパース                                        |
|   3. 送出した test_run_id、ログ件数、属性、トレースID、メッセージ本文の一致を検証    |
|   4. 詳細な検証レポートを表示し、成否ステータス (Exit 0 / 1) を返却               |
+-----------------------------------------------------------------------------------+
```

---

## 2. さくらのオブジェクトストレージ連携の重要ポイント（要件と罠）

### ① AWS SDK v2 チェックサム互換性フラグ（必須）
`sacloud-otel-collector` が内部で利用する AWS SDK for Go v2 は、デフォルトで最新の AWS S3 独自チェックサムヘッダー（`x-amz-checksum-crc32` 等）を付与して通信します。
さくらのオブジェクトストレージ等の S3 互換ストレージではこれらが未対応のため、**400 InvalidDigest** や **NotImplemented** エラーが発生する場合があります。
これを防ぐため、Collector の実行環境に以下の環境変数を必ず設定します：

```bash
export AWS_RESPONSE_CHECKSUM_VALIDATION=WHEN_REQUIRED
export AWS_REQUEST_CHECKSUM_CALCULATION=WHEN_REQUIRED
```

### ② Path-Style アクセスの強制 (`s3_force_path_style: true`)
さくらのオブジェクトストレージでは、バーチャルホスト形式（`bucket.s3.isk01...`）ではなく、パス形式（`s3.isk01.../bucket/key`）でアクセスすることで、SSL ワイルドカード証明書や名前解決の制約を受けることなく安定して通信できます。

### ③ エンドポイントとリージョン
- **石狩第1サイト (推奨)**:
  - エンドポイント: `https://s3.isk01.sakurastorage.jp`
  - リージョン名: `jp-north-1`
- **東京第1サイト**:
  - エンドポイント: `https://s3.tky01.sakurastorage.jp`
  - リージョン名: `jp-east-1`

---

## 3. 検証用ソフトウェア (`bin/verifier`) の構成

本リポジトリでは、100% Pure Go で開発された検証ツール `bin/verifier` を提供しています。

### 主なサブコマンド
| コマンド | 説明 |
| :--- | :--- |
| `bin/verifier roundtrip` (デフォルト) | ログ送出 ➔ バッチ Flush 待機 ➔ ストレージ読出 ➔ 一致検証を一括実行 |
| `bin/verifier emit` | テストログ（Run ID、TraceID、属性付き）を Collector へ送信 |
| `bin/verifier verify --run-id <ID>` | 指定した Run ID のログがストレージに届いているか検証 |
| `bin/verifier inspect` | バケット内のオブジェクト一覧および最新ログのデコード・表示 |

---

## 4. 検証方法 A: Mac ホスト上で直接実行（Docker 不要）

お使いの Mac（Apple Silicon `arm64` または Intel `amd64`）上で、Docker を介さず直接動作確認を行います。

### ステップ 1: バイナリのインストール
```bash
make install-collector-host
make build-verifier
```
*`bin/sacloud-otel-collector`（公式 macOS バイナリ）と `bin/verifier` が生成されます。*

### ステップ 2: `.env` ファイルの設定
リポジトリ直下に用意されているテンプレート `.env.example` をコピーして `.env` を作成し、さくらのオブジェクトストレージの認証情報を設定します：

```bash
cp .env.example .env
vi .env
```

**設定例 (`.env`)**:
```bash
SACLOUD_O2_ENDPOINT=https://s3.isk01.sakurastorage.jp
SACLOUD_O2_BUCKET=your-bucket-name
SACLOUD_O2_ACCESS_KEY=your-access-key
SACLOUD_O2_SECRET_KEY=your-secret-key
SACLOUD_O2_REGION=jp-north-1
SACLOUD_O2_PREFIX=logs
```

> [!NOTE]
> - `.env` および `.env.*` は `.gitignore` に登録されているため、API キーなどの秘密情報が Git リポジトリへ誤ってコミットされることはありません。
> - `make run-collector-host-sakura` および `make verify-otel-sakura`（ならびに `bin/verifier`）は、**起動時に自動的に `.env` を検知・読み込み**ます（`--env-file` で別ファイルを指定することも可能です）。

### ステップ 3: 1コマンドで全自動検証を実行 (★推奨)
別ターミナルを開く必要なく、以下の **1コマンド** を実行するだけで全自動検証が完了します：

```bash
make verify-sakura
```

このコマンドは以下のライフサイクルを自動制御します：
1. `.env` ファイルを自動ロード
2. `sacloud-otel-collector` を Mac のバックグラウンドで自動起動（ポート `4317` 待機、ログは `test_reports/collector.log` へ保存）
3. 検証ソフト `bin/verifier` が OTel トレース・ログを送出
4. さくらのオブジェクトストレージのバケットからオブジェクトを取得・解凍・内容突合
5. 検証結果レポートを表示
6. **完了時または中断時 (Ctrl+C) にバックグラウンドの Collector プロセスを自動停止・クリーンアップ**

*(従来通り 2 つのターミナルを分けて手動監視したい場合は、ターミナル 1 で `make run-collector-host-sakura`、ターミナル 2 で `make verify-otel-sakura` を実行することも可能です)*

**出力例**:
```
================================================================================
  OpenTelemetry ➔ Sakura Cloud Object Storage E2E Verification
================================================================================
  • Collector Endpoint : localhost:4317 (insecure: true)
  • Storage Endpoint   : https://s3.isk01.sakurastorage.jp
  • Target Bucket      : my-sacloud-bucket (prefix: logs)
  • Path-Style         : true | SSL: true
--------------------------------------------------------------------------------
==> [1/3] Initializing OTel Log Emitter (Collector: localhost:4317)...
==> [2/3] Emitting 5 structured log records with OTel Trace Context...
    ✓ Logs dispatched! RunID: run-20260921-192500-a1b2c3d4e5f60718
    ✓ Attached TraceID: 4bf92f3577b34da6a3ce929d0e0e4736
    ✓ Attached SpanID:  00f067aa0ba902b7
    ⏳ Waiting 4s for sacloud-otel-collector batch processor to flush to S3...
==> [3/3] Querying Sakura Cloud Object Storage (https://s3.isk01.sakurastorage.jp/my-sacloud-bucket)...

================================================================================
                             VERIFICATION REPORT                                
================================================================================
  Overall Status   : ✅ PASSED
  Test Run ID      : run-20260921-192500-a1b2c3d4e5f60718
  Emitted Records  : 5
  Matched Records  : 5
  Target Storage   : my-sacloud-bucket/logs
  Scan Duration    : 520ms
  Objects Scanned  : 1
  Matched Objects  : 1

  [Matched Storage Keys]
    • logs/2026/09/21/19/2026-09-21T10-25-04.json.gz

  [Verified Log Records Sample]
    [1] INFO  Verification log record 1 of 5 for run run-20260921-192500-a1b2c3d4e5f60718
        TraceID: 4bf92f3577b34da6a3ce929d0e0e4736 | SpanID: 00f067aa0ba902b7
        Seq: 1 | Time: 2026-09-21T10:25:00.123456789Z
    [2] WARN  Verification log record 2 of 5 for run run-20260921-192500-a1b2c3d4e5f60718 [warn-check]
        TraceID: 4bf92f3577b34da6a3ce929d0e0e4736 | SpanID: 00f067aa0ba902b7
        Seq: 2 | Time: 2026-09-21T10:25:00.123556789Z
================================================================================
```

---

## 5. 検証方法 B: Linux ホスト（VM）での systemd 常駐運用

さくらのクラウドの仮想サーバ（Ubuntu / RHEL / AlmaLinux）上でデーモンとして常駐運用する場合の手順です。

### 1. インストール
```bash
# Debian / Ubuntu の場合
wget https://github.com/sacloud/sacloud-otel-collector/releases/download/v0.8.0/sacloud-otel-collector_0.8.0_linux_amd64.deb
sudo dpkg -i sacloud-otel-collector_0.8.0_linux_amd64.deb

# RHEL / CentOS / AlmaLinux の場合
wget https://github.com/sacloud/sacloud-otel-collector/releases/download/v0.8.0/sacloud-otel-collector_0.8.0_linux_amd64.rpm
sudo rpm -i sacloud-otel-collector_0.8.0_linux_amd64.rpm
```

### 2. 環境変数設定ファイルの作成 (`/etc/default/sacloud-otel-collector`)
```bash
sudo mkdir -p /etc/default
sudo tee /etc/default/sacloud-otel-collector << 'EOF'
AWS_ACCESS_KEY_ID="<あなたのアクセスキー>"
AWS_SECRET_ACCESS_KEY="<あなたのシークレットキー>"
AWS_RESPONSE_CHECKSUM_VALIDATION="WHEN_REQUIRED"
AWS_REQUEST_CHECKSUM_CALCULATION="WHEN_REQUIRED"
SACLOUD_O2_ENDPOINT="https://s3.isk01.sakurastorage.jp"
SACLOUD_O2_BUCKET="<あなたのバケット名>"
SACLOUD_O2_REGION="jp-north-1"
SACLOUD_O2_PREFIX="logs"
EOF
sudo chmod 600 /etc/default/sacloud-otel-collector
```

### 3. 設定ファイルの配置 (`/etc/sacloud-otel-collector/config.yaml`)
リポジトリの `deploy/sacloud-otel-collector/config.sakura.yaml` を配置します：
```bash
sudo cp deploy/sacloud-otel-collector/config.sakura.yaml /etc/sacloud-otel-collector/config.yaml
```

### 4. systemd サービスの登録と起動
リポジトリ同梱の `deploy/sacloud-otel-collector/sacloud-otel-collector.service` を配置：
```bash
sudo cp deploy/sacloud-otel-collector/sacloud-otel-collector.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now sacloud-otel-collector
sudo systemctl status sacloud-otel-collector
```

### 5. 動作検証
Linux ホスト上で `bin/verifier` を実行して検証します：
```bash
./bin/verifier roundtrip \
  --s3-endpoint "https://s3.isk01.sakurastorage.jp" \
  --s3-bucket "<あなたのバケット名>" \
  --s3-access-key "<アクセスキー>" \
  --s3-secret-key "<シークレットキー>"
```

---

## 6. 検証方法 C: Docker Compose による完全ローカル検証

外部のさくらのオブジェクトストレージ契約が不要で、ローカルの MinIO（S3互換）コンテナと `sacloud-otel-collector` コンテナを連携させて全自動 E2E テストを実行します。

```bash
make verify-otel-local
```

このコマンドは以下を自動実行します：
1. `deploy/docker-compose.otel-s3.yml` を起動（MinIO + バケット作成 + Collector）。
2. Collector の起動を待機。
3. `bin/verifier roundtrip` を実行し、MinIO への書き込みと内容突合を検証。
4. 検証完了後にコンテナを自動停止・クリーンアップ。

---

## 7. 検証方法 D: Docker コンテナログ収集（`filelog` receiver）による検証 (★追加機能)

アプリケーションコンテナが排出する JSON 形式のログファイルを、`sacloud-otel-collector` の `filelog` receiver が tail 監視・パースし、さくらのオブジェクトストレージ（またはローカル MinIO）へ OTLP JSON (gzip) としてアーカイブ転送するエンドツーエンド検証です。

### アーキテクチャ構成
```
+-----------------------------------------------------------------------------------+
| アプリケーションコンテナ (log-producer)                                             |
|  - JSON 形式のコンテナログを共有ボリュームに出力                                    |
|    {"time":"...","stream":"stdout","log":"...","run_id":"...","seq":1}           |
+-----------------------------------------------------------------------------------+
                                          |
                                          | 共有ボリューム (sacloud-docker-app-logs)
                                          v
+-----------------------------------------------------------------------------------+
| sacloud-otel-collector (コンテナ)                                                  |
|                                                                                   |
|  - Receivers: filelog                                                             |
|      include: [/var/log/docker-app/*.log]                                         |
|      operators:                                                                   |
|        - type: json_parser (JSON フィールドを展開)                                 |
|        - type: time_parser (time フィールドから OTel Timestamp を生成)              |
|        - type: move (log フィールドを Body へ移動)                                 |
|        - type: add (source: docker_container 属性を付与)                           |
|                                                                                   |
|  - Exporters: awss3 (内包 exporter)                                               |
|      s3uploader:                                                                  |
|        endpoint: https://s3.tky01.sakurastorage.jp (または MinIO)                 |
|        s3_bucket: ${SACLOUD_O2_BUCKET}                                            |
|        s3_prefix: logs                                                            |
|      compression: gzip                                                            |
|      marshaler: otlp_json                                                         |
+-----------------------------------------------------------------------------------+
                                          |
                                          | S3 API (PUT logs/YYYY/MM/DD/HH/*.json.gz)
                                          v
+-----------------------------------------------------------------------------------+
| さくらのオブジェクトストレージ / MinIO                                              |
|  - OTLP JSON (gzip) アーカイブとして永続化                                         |
+-----------------------------------------------------------------------------------+
                                          ^
                                          | S3 API (GetObject, 突合アサーション)
+-----------------------------------------+-----------------------------------------+
| 検証ソフトウェア (bin/verifier verify)                                             |
|  - S3 オブジェクトから対象 RunID と全ログレコードの一致を検証                       |
+-----------------------------------------------------------------------------------+
```

### 実行コマンド (1コマンド全自動)

#### ① さくらのオブジェクトストレージ実環境（東京第1サイト等）
```bash
make verify-docker-log-sakura
```
*`.env` の認証情報を読み込み、Compose 起動 ➔ ログ排出 ➔ Collector 収集 ➔ S3 転送 ➔ 検証 ➔ 自動クリーンアップまでを 1 コマンドで実行します。*

#### ② ローカル環境（MinIO）
```bash
make verify-docker-log-local
```
*外部接続不要で、ローカル MinIO と Collector を立ち上げて同様の完全自動検証を行います。*

> [!TIP]
> **タイムゾーン（JST）について**:
> コンテナ環境ではデフォルトで UTC が使われるため、S3 の `%Y/%m/%d/%H` パーティションが UTC 時間（例: 23時は 14時）で切られます。
> 本リポジトリの Compose 設定（`deploy/docker-compose.docker-log.yml`）および Collector イメージには `tzdata` と `TZ=Asia/Tokyo` を標準設定しているため、**日本標準時（JST）ベースのフォルダ構成（例: `logs/2026/09/21/23/`）** で直感的に格納されます。

### 検証レポート出力例
```
================================================================================
                             VERIFICATION REPORT                                
================================================================================
  Overall Status   : ✅ PASSED
  Test Run ID      : docker-sakura-20260921-233803-25951
  Emitted Records  : 5
  Matched Records  : 5
  Target Storage   : sh0jitmy-sacloud-otelcollector-verify/logs
  Scan Duration    : 412ms
  Objects Scanned  : 6
  Matched Objects  : 1

  [Matched Storage Keys]
    • logs/2026/09/21/23/logs_579339224.json.gz

  [Verified Log Records Sample]
    [1]       Docker container log record 1 of 5 for run docker-sakura-20260921-233803-25951
        TraceID:  | SpanID: 
        Seq: 1.000000 | Time: 2026-09-21T14:38:04Z
    [2]       Docker container log record 2 of 5 for run docker-sakura-20260921-233803-25951
        TraceID:  | SpanID: 
        Seq: 2.000000 | Time: 2026-09-21T14:38:04Z
================================================================================
🎉 Docker container log verification completed successfully!
```

---

## 8. トラブルシューティング

| 現象 | 主な原因 | 対処方法 |
| :--- | :--- | :--- |
| `PutObject: 400 InvalidDigest` または `NotImplemented` | AWS SDK v2 のストリーミングチェックサムヘッダーが送信されている | `AWS_REQUEST_CHECKSUM_CALCULATION=WHEN_REQUIRED` および `AWS_RESPONSE_CHECKSUM_VALIDATION=WHEN_REQUIRED` を設定する |
| `NoSuchBucket` または `404 Not Found` | バケット名が未作成または誤っている | コントロールパネルでバケットを作成し、バケット名を確認する |
| `connection refused` (4317) | Collector が起動していない、またはポートがバインドされていない | `sacloud-otel-collector` が実行中か、ポート 4317 をリスンしているか確認する |
| タイムアウト（ログが見つからない） | Batch Processor の flush 待ち時間が不足している | `--flush-wait 5s` や `--max-attempts 15` で待機時間を長めに設定する |
| `denied: denied` (ghcr.io image pull) | ghcr.io に公式コンテナイメージが公開されていない | リポジトリ同梱の `deploy/sacloud-otel-collector/Dockerfile` により自動ローカルビルドを行う（設定済み） |
| S3上のフォルダ（時）が日本時間とずれる | コンテナのタイムゾーンが UTC（協定世界時）になっている | コンテナに `tzdata` を導入し、環境変数 `TZ=Asia/Tokyo` を設定する（設定済み） |
