.PHONY: help install build run-server run-dev dev clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install frontend dependencies
	cd web && npm install

build: ## Build the frontend for production
	cd web && npm run build

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




