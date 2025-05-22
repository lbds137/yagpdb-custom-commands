# YAGPDB Custom Commands - Development Tools

.PHONY: help lint lint-verbose build-linter clean test

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build-linter: ## Build the linter tool
	@echo "🔨 Building linter..."
	@cd tools/linter && go build -o ../../bin/yagpdb-lint .
	@echo "✅ Linter built successfully"

lint: build-linter ## Run linter on all .gohtml files
	@echo "🔍 Running linter..."
	@./bin/yagpdb-lint -dir .

lint-verbose: build-linter ## Run linter with verbose output
	@echo "🔍 Running linter (verbose)..."
	@./bin/yagpdb-lint -dir . -v

lint-fix: build-linter ## Run linter with auto-fix (when available)
	@echo "🔧 Running linter with auto-fix..."
	@./bin/yagpdb-lint -dir . -fix

clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/
	@echo "✅ Clean complete"

test: ## Run tests (placeholder for future implementation)
	@echo "🧪 Running tests..."
	@echo "⚠️  Tests not yet implemented"

# Create bin directory if it doesn't exist
bin:
	@mkdir -p bin

# Ensure bin directory exists before building
build-linter: | bin

# Development shortcuts
dev-lint: lint-verbose ## Alias for lint-verbose

# Reporting targets
lint-report: ## Generate and save lint report with timestamp
	@echo "📊 Generating lint report..."
	@./save-lint-report.py

lint-history: ## Show available lint reports
	@echo "📚 Available lint reports:"
	@ls -la reports/lint_output_*.txt 2>/dev/null || echo "No reports found"

lint-latest: ## Show latest lint report
	@echo "📋 Latest lint report:"
	@cat reports/latest_lint.txt 2>/dev/null || echo "No latest report found"

# CI/CD targets
ci-lint: build-linter ## Run linter for CI (stricter)
	@echo "🤖 Running CI linter..."
	@./bin/yagpdb-lint -dir . || (echo "❌ Linting failed in CI" && exit 1)

ci-report: ## Generate lint report for CI
	@echo "📊 Generating CI lint report..."
	@./lint-report.py --latest --markdown

# Installation target for development
install-dev: build-linter ## Install linter to system PATH
	@echo "📦 Installing linter to system..."
	@cp bin/yagpdb-lint /usr/local/bin/ 2>/dev/null || echo "⚠️  Could not install to /usr/local/bin (try with sudo)"
	@echo "✅ Installation complete (if no errors above)"