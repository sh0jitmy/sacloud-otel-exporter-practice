---
name: observability-stack
description: "VictoriaMetrics、Prometheus、Grafana、OpenTelemetry（トレース & メトリクス）による統合可観測性スタックの構築、設定、ダッシュボードプロビジョニング、およびE2Eメトリクス検証プラクティス。"
user-invocable: true
license: Apache-2.0
compatibility: Designed for Claude Code, Cursor, Antigravity, OpenCode, OpenClaw, and other AI coding agents.
metadata:
  author: [YOUR_NAME]
  version: "1.0.0"
  openclaw:
    emoji: "📈"
    homepage: https://github.com/shjtmy/go_sh0jitmy_template
    requires:
      bins:
        - go
        - docker
    install: []
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Bash(docker:*) Agent AskUserQuestion
---

# Modern Observability Stack (VictoriaMetrics + Grafana + OpenTelemetry)

## 1. アーキテクチャ構成
```
+--------------------+        Scrape        +--------------------+
| Go Template App    | <------------------- | VictoriaMetrics    |
| - OTel Tracing     |                      | (Prometheus-compat)|
| - Prometheus /metrics                     +--------------------+
| - pprof :6060      |                                |
+--------------------+                                v
          |                                  +--------------------+
          +--------------------------------> | Grafana Dashboard  |
                PostgreSQL Direct Datasource | (Overview UI)      |
                                             +--------------------+
```

## 2. OpenTelemetry & Prometheus 計装
- `go.opentelemetry.io/otel` による分散トレーシング。
- `otelprom.New()` による Prometheus Exporter の登録。
- 標準メトリクス:
  - `go_goroutines`: アクティブな Goroutine 数
  - `go_memstats_alloc_bytes`: ヒープ割り当てメモリ量
  - `process_cpu_seconds_total`: プロセス CPU 使用率
  - `http_requests_total`: HTTP リクエスト総数

## 3. VictoriaMetrics による軽量スクレイピング
- `deploy/victoriametrics/prometheus.yml`:
  ```yaml
  global:
    scrape_interval: 5s
  scrape_configs:
    - job_name: "go-template-app"
      static_configs:
        - targets: ["app:8080"]
  ```

## 4. Grafana 自動プロビジョニング
- `deploy/grafana/datasources.yaml`: VictoriaMetrics (Prometheus型) および PostgreSQL データソースを自動定義。
- `deploy/grafana/dashboards.yaml`: プロビジョニング設定。
- `deploy/grafana/dashboards/overview.json`: システムリソース、メトリクス推移、ユーザーインベントリ、監査ログを一画面に統合したダッシュボード。
