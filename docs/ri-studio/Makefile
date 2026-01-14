.PHONY: help install build build-wasm build-static run-server run-dev dev clean clean-wasm

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install frontend dependencies
	cd web && npm install

build: ## Build the frontend for production (server-based)
	cd web && npm run build

build-wasm: ## Build the WASM binary and copy wasm_exec.js
	@echo "Syncing examples from docs/examples/ (go:embed doesn't follow symlinks)..."
	@mkdir -p wasm/examples
	@rsync -a --delete ../examples/ wasm/examples/
	@echo "Building WASM binary..."
	cd wasm && go mod tidy && GOOS=js GOARCH=wasm go build -o ../web/public/wasm.wasm .
	@echo "Copying wasm_exec.js from Go SDK..."
	@if [ -f "$$(go env GOROOT)/misc/wasm/wasm_exec.js" ]; then \
		cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" web/public/wasm_exec.js; \
	elif [ -f "$$(go env GOROOT)/lib/wasm/wasm_exec.js" ]; then \
		cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/public/wasm_exec.js; \
	else \
		echo "Error: wasm_exec.js not found in Go SDK"; \
		exit 1; \
	fi
	@echo "WASM build complete!"

build-static: build-wasm install ## Build complete static site with WASM
	@echo "Building static frontend..."
	cd web && npm run build
	@echo "Static build complete! Deploy web/dist directory."

run-server: ## Run the Go backend server (requires built frontend)
	cd server && go run main.go

run-dev: ## Run both backend and frontend in dev mode (requires two terminals)
	@echo "Run these commands in separate terminals:"
	@echo "  Terminal 1: make run-server"
	@echo "  Terminal 2: cd web && npm run dev"

dev: ## Start development mode (backend only, use 'cd web && npm run dev' for frontend)
	cd server && go run main.go

clean: ## Clean build artifacts
	rm -rf web/dist web/node_modules
	rm -f server/ri-studio

clean-wasm: ## Clean WASM artifacts
	rm -f web/public/wasm.wasm web/public/wasm_exec.js wasm/wasm.wasm




