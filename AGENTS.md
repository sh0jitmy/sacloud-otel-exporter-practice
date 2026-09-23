# AGENTS.md - Antigravity Agent Guidelines

This document provides behavioral constraints, architectural conventions, and execution workflows for Google Antigravity and other AI coding agents working in this repository.

## Repository Overview

`sacloud-otel-exporter-practice` provides:
1. **Sakura Cloud Object Storage × OpenTelemetry Collector Integration**: S3-compatible log streaming via `sacloud-otel-collector` (`awss3` exporter) to Sakura Cloud Object Storage (`tky01` / `isk01`), Docker container log pipeline (`filelog` receiver), and JST timezone partitioning (`logs/%Y/%m/%d/%H/`).
2. **Pure Go Verification Tooling**: `cmd/verifier` & `internal/otels3` providing automated 1-command verification (`make verify-sakura`, `make verify-docker-log-sakura`) with gzip decompression and OTLP / Docker JSON ID assertion.
3. **Zero-npm Standalone HTMX Dashboard & SSG**: High-performance UI server (`internal/web`) using `//go:embed`, local HTMX (`static/js/htmx.min.js`), and pre-rendered static site export.
4. **SQLite Governance & Reliability**: CGO-free WAL-mode persistence, SHA-256 verified backup archives (`tar.gz`), atomic transactional restore, and retention cleaner.
5. **Multi-Tier E2E Testing Framework**: Fast No-Docker SQLite E2E (`make sqlite-e2e`), Headless Chrome frontend UI assertions (`make frontend-e2e`), Docker Compose full-stack E2E (`make docker-e2e`), and S3 storage E2E (`make verify-sakura`).
6. **Production Observability**: VictoriaMetrics, Prometheus metric endpoint (`/metrics`), pprof (`127.0.0.1:6060`), and pre-provisioned Grafana dashboards (`deploy/grafana/`).
7. **SSOT Version Management**: Version centrally defined in `internal/version/version.go`, Go version unified across GitHub Actions and `go.mod` via `go-version-file: 'go.mod'`, release automation via GoReleaser v2.

---

## Agent Behavioral Rules

1. **Custom Skills First**:
   - Relevant skills are located in `.agents/skills/` and `.claude/skills/`.
   - Adhere to `golang-htmx-frontend`, `golang-sqlite-governance`, `multi-tier-e2e-testing`, and `observability-stack` for architecture and implementation decisions.
2. **License Header Integrity**:
   - Every Go source file must contain the standard Apache-2.0 and Author header.
   - Always verify with `make license-check` before concluding tasks.
3. **SSOT Principle**:
   - Never hardcode Go versions in GitHub Actions workflows; always use `go-version-file: 'go.mod'`.
   - Version injection must go through `internal/version.Version`.
4. **Test Isolation**:
   - In Go unit/integration tests (`internal/...`), isolate SQLite memory databases using distinct connection names (`fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())`) to ensure parallel safety.
5. **Requirements Compliance**:
   - After updating features or configurations, execute `make self-eval` and ensure 100% compliance in `REQUIREMENTS.md`.

---

## Standard Development & Testing Commands

```bash
# Code generation & formatting
make generate
make fmt
make lint

# Sakura Cloud Object Storage & OTel Verification
make verify-sakura            # 1-command all-in-one Sakura Cloud Object Storage verification
make verify-docker-log-sakura # Docker container log pipeline verification to Sakura Cloud
make verify-otel-local        # LocalStack S3 verification (Local E2E)
make verify-docker-log-local  # LocalStack Docker log pipeline verification

# Verification & Multi-Tier E2E
make test            # Unit tests with -race and coverage check
make sqlite-e2e      # Ultra-fast No-Docker SQLite E2E
make frontend-e2e    # Headless Chrome HTMX UI & snapshot verification
make docker-e2e      # Multi-container Docker Compose E2E
make ssg-build       # Static site generation export

# Release & Governance
make release-check   # Validate GoReleaser v2 configuration
make license-check   # Verify license headers
make check           # Validate skill frontmatter
make self-eval       # Update REQUIREMENTS.md checklist & compliance score
```
