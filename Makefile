# Makefile for Go Development & Custom Skills Management

.PHONY: help check install install-agents install-all self-eval generate test fmt lint tidy vulncheck build release-check release-snapshot license-check license-add migration-diff clean openapi-lint publish-pr ai-pr run sqlite-e2e frontend-e2e docker-e2e ssg-build demo

help:
	@echo "Available commands:"
	@echo "  Go Development & Testing:"
	@echo "    openapi-lint     Validate OpenAPI spec with Spectral"
	@echo "    generate         Generate OpenAPI and ent entity code"
	@echo "    fmt              Format Go source files"
	@echo "    lint             Run golangci-lint static analysis"
	@echo "    tidy             Run go mod tidy"
	@echo "    vulncheck        Run govulncheck vulnerability scanner"
	@echo "    test             Run Go tests with race detector and coverage"
	@echo "    build            Build binaries to bin/app and bin/web"
	@echo "    run              Run local standalone stack (Core API + Web Dashboard)"
	@echo "    sqlite-e2e       Run fast standalone SQLite E2E test (No-Docker)"
	@echo "    frontend-e2e     Run standalone HTMX frontend E2E test & snapshot suite"
	@echo "    docker-e2e       Run full-stack Docker Compose E2E test & Grafana assertions"
	@echo "    ssg-build        Generate pre-rendered static site HTML and assets (SSG)"
	@echo "    demo             Launch full-stack interactive demo with seeded data"
	@echo "    release-check    Validate GoReleaser configuration"
	@echo "    release-snapshot Run GoReleaser snapshot build"
	@echo "    license-check    Verify license & author headers in Go files"
	@echo "    license-add      Automatically add license headers to Go files"
	@echo "    migration-diff   Generate DB migration SQL file with Atlas"
	@echo "    publish-pr       Verify formatting/lints/tests, push to origin, and create GitHub PR"
	@echo "    ai-pr            Trigger AI agent to draft a GitHub PR in Japanese"
	@echo "  Custom Skills Management:"
	@echo "    check            Validate custom skill frontmatter and syntax"
	@echo "    install          Install custom skills globally to ~/.claude/skills/"
	@echo "    install-agents   Install/Sync custom skills to .agents/skills/ (Antigravity)"
	@echo "    install-all      Install custom skills to both Claude and Antigravity"
	@echo "    self-eval        Run requirements self-evaluation and update checklist"
	@echo "  General:"
	@echo "    clean            Clean up build artifacts and temporary files"

# --- Go Development ---

openapi-lint:
	@echo "==> Running Spectral lint on OpenAPI spec..."
	@if command -v spectral >/dev/null 2>&1; then \
		NODE_OPTIONS="--no-deprecation" spectral lint api/openapi.yaml; \
	elif command -v npx >/dev/null 2>&1; then \
		NODE_OPTIONS="--no-deprecation" npx -y @stoplight/spectral-cli lint api/openapi.yaml; \
	else \
		echo "Spectral CLI is not installed and npx is not available. Please install it."; \
		exit 1; \
	fi

generate: openapi-lint
	@echo "==> Generating code from schema..."
	@go generate ./...

fmt: generate
	@echo "==> Formatting Go source files..."
	@go fmt ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix ./...; \
	fi

lint: generate
	@echo "==> Running golangci-lint..."
	@golangci-lint run ./...

tidy:
	@echo "==> Tidying Go modules..."
	@go mod tidy

vulncheck:
	@echo "==> Running govulncheck..."
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...

test: generate
	@bash scripts/check_coverage.sh

build: generate
	@echo "==> Building binaries..."
	@mkdir -p bin
	@go build -v -o bin/app ./cmd/app
	@go build -v -o bin/web ./cmd/web

run: build
	@echo "==> Starting local standalone servers..."
	@bash scripts/run_local.sh

sqlite-e2e: build
	@echo "==> Running Standalone SQLite E2E tests..."
	@bash scripts/sqlite_e2e.sh

frontend-e2e: build
	@echo "==> Running Standalone HTMX Frontend E2E tests..."
	@bash scripts/frontend_e2e.sh

docker-e2e:
	@echo "==> Running Full-Stack Docker Compose E2E tests..."
	@bash scripts/docker_e2e.sh

ssg-build:
	@echo "==> Generating static site export (SSG)..."
	@mkdir -p dist/static-site
	@go run ./cmd/web --ssg-export dist/static-site

demo:
	@echo "==> Starting Full-Stack Live Demo..."
	@bash scripts/demo.sh

release-check:
	@echo "==> Validating GoReleaser configuration..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser check; \
	else \
		go run github.com/goreleaser/goreleaser/v2@latest check; \
	fi

release-snapshot:
	@echo "==> Building GoReleaser snapshot..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean; \
	fi

license-check:
	@echo "==> Checking Go source files license headers..."
	@python3 scripts/check_license.py --check

license-add:
	@echo "==> Adding license headers to Go source files..."
	@python3 scripts/check_license.py --add

migration-diff:
	@if [ -z "$(name)" ]; then \
		echo "Error: name is required. Usage: make migration-diff name=migration_name"; \
		exit 1; \
	fi
	@if ! command -v atlas >/dev/null 2>&1; then \
		echo "Atlas CLI is not installed. Please install it from: https://atlasgo.io/"; \
		exit 1; \
	fi
	@echo "==> Generating DB migration DDL with Atlas..."
	@mkdir -p ent/migrate/migrations
	@atlas migrate diff $(name) \
		--dir "file://ent/migrate/migrations" \
		--to "ent://ent/schema" \
		--dev-url "sqlite://dev?mode=memory"

publish-pr:
	@bash scripts/publish_pr.sh

ai-pr:
	@claude "github-pr-creator スキルを使用して、現在のブランチの変更とコミットログを分析し、pull_request_template.md に従って日本語のプルリクエストをドラフト（下書き）で作成してください。"

# --- Custom Skills Management ---

check:
	@echo "==> Validating skill files format..."
	@python3 scripts/check_skills.py

install:
	@echo "==> Installing custom skills globally to ~/.claude/skills/..."
	@mkdir -p ~/.claude/skills/
	@cp -R .claude/skills/* ~/.claude/skills/
	@echo "Claude skills successfully installed!"

install-agents:
	@echo "==> Syncing custom skills to .agents/skills/ (Antigravity)..."
	@mkdir -p .agents/skills/
	@cp -R .claude/skills/* .agents/skills/
	@echo "Antigravity skills successfully synced!"

install-all: install install-agents
	@echo "All custom skills successfully installed for Claude and Antigravity!"

self-eval:
	@echo "==> Running self-evaluation..."
	@python3 scripts/self_eval.py

# --- General ---

clean:
	@echo "==> Cleaning up build artifacts..."
	@rm -rf bin/ dist/ ent/migrate/migrations/ test_reports/
	@go clean -testcache
