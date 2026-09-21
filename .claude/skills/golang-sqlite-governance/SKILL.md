---
name: golang-sqlite-governance
description: "GoにおけるSQLiteのプロダクション運用、WALモード設定、接続プール最適化、SHA256ハッシュ検証付きバックアップ＆アトミックリストア、およびデータ保持期間パージ（Retention Cleaner）のガバナンス。"
user-invocable: true
license: Apache-2.0
compatibility: Designed for Claude Code, Cursor, Antigravity, OpenCode, OpenClaw, and other AI coding agents.
metadata:
  author: [YOUR_NAME]
  version: "1.0.0"
  openclaw:
    emoji: "🗄️"
    homepage: https://github.com/shjtmy/go_sh0jitmy_template
    requires:
      bins:
        - go
    install: []
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Agent AskUserQuestion
---

# Go SQLite Governance & Enterprise Reliability

## 1. DSN 接続パラメータの必須要件
SQLite を並行実行環境（Webサーバー、API）で安全に使用するため、以下の PRAGMA パラメータを常に適用する:
```
file:app.db?cache=shared&mode=rwc&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)
```
- `foreign_keys(1)`: 外部キー制約の強制。ent の自動マイグレーションで必須。
- `journal_mode(WAL)`: Write-Ahead Logging による同時読み取りと書き込みの競合低減。
- `busy_timeout(5000)`: ロック競合時の 5000ms リトライ待機。

## 2. コネクションプール設計
- SQLite は同一ファイルへの同時書き込みが単一スレッドに制限されるため、接続プールを適切に制御する:
  ```go
  db.SetMaxOpenConns(1) // 単一接続モードでロック競合を完全防止
  // または WAL モード下で読み取り並行を許容する場合:
  db.SetMaxOpenConns(10)
  db.SetMaxIdleConns(5)
  ```

## 3. バックアップ・リストアアーキテクチャ
- **アトミック VACUUM INTO**: SQLite 3.27+ の `VACUUM INTO 'backup.db'` を用いて、ロック中のデータベースから安全に一貫したスナップショットを抽出。
- **改変検知マニフェスト (`manifest.json`)**:
  - `backup_id`, `created_at`, `db_size_bytes`, `db_sha256` を記録。
  - バックアップアーカイブ全体を `tar.gz` に圧縮。
- **改変・破損耐性リストア**:
  - アーカイブ解凍後、データベースファイルの SHA256 ハッシュを計算しマニフェストと照合。
  - ハッシュ不一致時はリストアを即座に中止し、既存データを破損させない。

## 4. データ保持期間クリーナー (Retention Cleaner)
- 古いバックアップファイルや時系列監査レコードの自動削除:
  `PurgeExpiredRecords(ctx, retentionDays)`
  保持期間を超過した古いレコードやアーカイブを安全にトランザクション内でパージする。
