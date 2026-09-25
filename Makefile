# YAGPDB Custom Commands - Development Tools

.PHONY: help lint lint-verbose build-emulator clean test test-verbose update-snapshots prune-snapshots test-go watch ci test-templates changed-since-deploy mark-deployed

LINTER := python3 tools/linter/yagpdb_lint.py
YAGTEST_FLAGS := -schema db_schema.yaml

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-22s %s\n", $$1, $$2}'

build-emulator: ## Build the template emulator/test runner
	@echo "🔨 Building emulator..."
	@cd tools/emulator && go build -o ../../bin/yagtest ./cmd/yagtest
	@echo "✅ Emulator built successfully"

lint: ## Run linter on all .gohtml commands
	@echo "🔍 Running linter..."
	@$(LINTER) --dir .

lint-verbose: ## Run linter with verbose output
	@echo "🔍 Running linter (verbose)..."
	@$(LINTER) --dir . -v

clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/
	@echo "✅ Clean complete"

test: build-emulator ## Run all template tests
	@echo "🧪 Running template tests..."
	@./bin/yagtest test $(YAGTEST_FLAGS) tools/emulator/testdata/

test-verbose: build-emulator ## Run template tests with verbose output
	@echo "🧪 Running template tests (verbose)..."
	@./bin/yagtest test -verbose $(YAGTEST_FLAGS) tools/emulator/testdata/

test-go: ## Run the emulator's Go unit tests and vet
	@echo "🧪 Running Go tests..."
	@cd tools/emulator && go vet ./... && go test ./...

update-snapshots: build-emulator ## Rewrite snapshots after an intended output change
	@./bin/yagtest test -update-snapshots $(YAGTEST_FLAGS) tools/emulator/testdata/

prune-snapshots: build-emulator ## Remove only the snapshots of renamed or deleted tests
	@./bin/yagtest test -prune-snapshots $(YAGTEST_FLAGS) tools/emulator/testdata/

watch: build-emulator ## Rerun template tests whenever a command or test changes
	@./bin/yagtest watch $(YAGTEST_FLAGS) -watch tools/emulator/testdata,utility,staff_utility,docs/cookbook tools/emulator/testdata/

# As on GitHub (which sets CI): a missing or stale snapshot fails instead of being written
# or only warned about
ci: export CI := true
ci: test-go test test-templates lint ## Everything CI runs
	@echo "🔍 Checking Go formatting..."
	@test -z "$$(gofmt -l tools/emulator)" || (gofmt -l tools/emulator && echo "❌ Run: gofmt -w tools/emulator" && exit 1)
	@echo "✅ All checks passed"

test-templates: build-emulator ## Smoke test: run every command once with no arguments
	@./scripts/test-all-templates.sh

analyze: ## Analyze templates for missing emulator functions
	@./scripts/find-missing-functions.sh

# Create bin directory if it doesn't exist
bin:
	@mkdir -p bin

# Ensure bin directory exists before building
build-emulator: | bin

# Development shortcuts
dev-lint: lint-verbose ## Alias for lint-verbose

# Reporting targets
lint-report: ## Generate and save lint report with timestamp
	@echo "📊 Generating lint report..."
	@./scripts/save-lint-report.py

lint-history: ## Show available lint reports
	@echo "📚 Available lint reports:"
	@ls -la reports/lint_output_*.txt 2>/dev/null || echo "No reports found"

lint-latest: ## Show latest lint report
	@echo "📋 Latest lint report:"
	@cat reports/latest_lint.txt 2>/dev/null || echo "No latest report found"

# CI/CD targets
ci-report: ## Generate lint report for CI
	@echo "📊 Generating CI lint report..."
	@./scripts/lint-report.py --latest --markdown
# Deployment: commands are pasted into the YAGPDB control panel by hand
COMMAND_DIRS := guests utility staff_utility

changed-since-deploy: ## List command files changed since the `deployed` tag (paste these)
	@git rev-parse -q --verify deployed >/dev/null || (echo "No 'deployed' tag yet: run make mark-deployed after a paste" && exit 1)
	@git diff --name-only --diff-filter=AM deployed -- $(COMMAND_DIRS) | grep '\.gohtml$$' || echo "Nothing to paste: no command changed since $$(git log -1 --format='%h %as' deployed)"
	@git diff --name-only --diff-filter=D deployed -- $(COMMAND_DIRS) | grep '\.gohtml$$' | sed 's/^/deleted (remove from YAGPDB): /' || true

mark-deployed: ## Record that the current commit's commands are live in YAGPDB
	@git tag -f deployed HEAD >/dev/null && echo "deployed → $$(git log -1 --format='%h %s' HEAD)"
	@git push -q -f origin deployed 2>/dev/null && echo "Tag pushed" || echo "Tag not pushed (offline?); run: git push -f origin deployed"
