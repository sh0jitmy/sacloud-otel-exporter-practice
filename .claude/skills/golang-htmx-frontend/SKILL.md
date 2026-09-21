---
name: golang-htmx-frontend
description: "Go言語組み込みの //go:embed、html/template、およびHTMXを活用した、Node.js/npm非依存の超軽量・高性能なスタンドアロンWebダッシュボードとSSG（静的サイト生成）の実装プラクティス。"
user-invocable: true
license: Apache-2.0
compatibility: Designed for Claude Code, Cursor, Antigravity, OpenCode, OpenClaw, and other AI coding agents.
metadata:
  author: [YOUR_NAME]
  version: "1.0.0"
  openclaw:
    emoji: "⚡"
    homepage: https://github.com/shjtmy/go_sh0jitmy_template
    requires:
      bins:
        - go
    install: []
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Agent AskUserQuestion
---

# Go + HTMX Standalone Frontend Architecture

## 1. コア原則
- **Node.js/npm 非依存**: フロントエンドアセット（HTMX、CSS、SVGアイコン）をリポジトリ内に直接内包（Air-gapped環境対応）し、外部CDNや重厚なビルドツールへの依存をゼロにする。
- **Go組み込み Single Binary 配備**: `//go:embed` ディレクティブを用いてHTMLテンプレートおよび静的アセットをGoバイナリに直接コンパイル。単一バイナリ配布を堅持する。
- **Hypermedia-Driven**: クライアント側のSPAフレームワークを使わず、サーバーサイドレンダリング（SSR）によるHTMLパーシャル返却とHTMXの属性（`hx-get`, `hx-post`, `hx-target`, `hx-swap`）を活用する。

## 2. テンプレート構造とコンポーネント分離
```
internal/web/
├── ssg.go                 # 静的サイト事前レンダリング出力 (SSG)
├── ui_server.go           # HTTPサーバー & HTMXハンドラー
├── static/                # 静的アセット (CSS, JS)
│   ├── css/
│   │   └── dashboard.css
│   └── js/
│       └── htmx.min.js
└── templates/             # html/template
    ├── layout.html        # ベースレイアウト (ダークモード・レスポンシブ)
    ├── dashboard.html     # メインビュー
    └── components/        # HTMXで非同期ロード・更新されるパーシャル
        ├── backup_panel.html
        ├── system_metrics.html
        └── users_table.html
```

## 3. HTMXの実装パターン
- **定期ポーリング (System Metrics)**:
  `hx-get="/ui/components/system-metrics" hx-trigger="load, every 5s" hx-swap="innerHTML"`
- **ユーザー操作によるインプレース更新 (Create Backup)**:
  `hx-post="/ui/actions/create-backup" hx-target="#backup-list-container" hx-swap="innerHTML"`
- **アクティブ検索・フィルタリング**:
  `hx-post="/ui/components/search" hx-trigger="keyup changed delay:300ms" hx-target="#results"`

## 4. SSG (Static Site Generation)
- `ExportStaticSite(outDir string, data PageData) error`
- CI/CDパイプラインやオフライン監査用、GitHub Pagesデプロイ用に、同一のGoテンプレートを用いて静的HTMLおよびアセットを瞬時に事前レンダリング出力する。
