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

# ADR 0003: スタンドアロン HTMX フロントエンド & SSG アーキテクチャの採用

## ステータス
承認済み (Accepted)

## コンテキスト
従来のマイクロサービスや社内管理ツールでは、SPA（React、Vue、Next.js など）を採用することで、Node.js や npm の巨大な依存関係、脆弱性対応、複雑なビルドパイプラインが必要となり、保守コストや閉域網（Air-gapped）環境へのデプロイ障壁が増大していました。
軽量かつ高速で、Node.js 非依存でありながら、リッチでインタラクティブなダッシュボードと静的サイト生成（SSG）を Go バイナリ単体で提供する設計が求められていました。

## 意思決定

以下の設計方針に基づき、HTMX と Go 組み込み機能によるスタンドアロンフロントエンドを導入します。

1. **Node.js / npm の完全排除とローカルアセット内包**:
   - `internal/web/static/js/htmx.min.js` をリポジトリ内に直接内包。外部 CDN へのアクセスを行わず、閉域網・Air-gapped 環境でも完全にオフライン動作可能。
2. **`//go:embed` による単一バイナリ配布**:
   - HTML テンプレート（`internal/web/templates/`）および静的アセット（`internal/web/static/`）をコンパイル時にバイナリに直接埋め込み、単一バイナリ配布（Single-binary deployment）を実現。
3. **Hypermedia-Driven アーキテクチャ (HTMX)**:
   - システムメトリクスの定期自動ポーリング（`hx-get="/ui/components/system-metrics" hx-trigger="load, every 5s"`）。
   - ユーザー操作に応じたインプレース更新（バックアップ即時作成、パーシャル HTML の置換）。
4. **SSG (Static Site Generation) のデュアルモード対応**:
   - `ExportStaticSite` 関数および `--ssg-export` フラグにより、Web サーバーを常駐起動せずとも、同一の Go テンプレートから静的 HTML とアセットを事前レンダリング出力可能。GitHub Pages や監査用アーカイブに即時活用可能。
5. **バイナリ構成 (案A)**:
   - メインバイナリ `cmd/app`（`server`, `web` サブコマンド）を中心としつつ、独立ビルド・実行可能な `cmd/web` も提供。

## 帰結

- **利点**:
  - npm/Node.js のインストール・セキュリティパッチ対応が不要。
  - メモリフットプリントが極めて小さく（< 20MB）、高速起動（< 10ms）。
  - SSG により、サーバーレス環境や静的ホスティングへも即座に展開可能。
- **留意点**:
  - クライアント側での極度に複雑なグラフィックスや重厚なキャンバス描画を行う場合は、Vanilla JS や軽量ライブラリの追加検討が必要。
