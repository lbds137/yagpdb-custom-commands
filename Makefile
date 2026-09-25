# YAGPDB Custom Commands - Development Tools

.PHONY: help lint lint-verbose build-emulator clean test test-verbose update-snapshots test-go watch ci test-templates

LINTER := python3 tools/linter/yagpdb_lint.py
YAGTEST_FLAGS := -schema db_schema.yaml

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

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

watch: build-emulator ## Rerun template tests whenever a command or test changes
	@./bin/yagtest watch $(YAGTEST_FLAGS) -watch tools/emulator/testdata,utility,staff_utility tools/emulator/testdata/

ci: test-go test lint ## Everything CI runs
	@echo "🔍 Checking Go formatting..."
	@test -z "$$(gofmt -l tools/emulator)" || (gofmt -l tools/emulator && echo "❌ Run: gofmt -w tools/emulator" && exit 1)
	@echo "✅ All checks passed"

test-templates: build-emulator ## Test all .gohtml templates against emulator
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