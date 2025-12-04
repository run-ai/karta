# RI Studio - Quick Start Guide

Get started with RI Studio in just a few minutes!

## Prerequisites

- **Go 1.24+**: `go version`
- **Node.js 18+**: `node --version`
- **npm**: `npm --version`

## Option 1: Quick Start (Production Mode)

The fastest way to get started:

```bash
cd docs/ri-studio
./start.sh
```

Then open http://localhost:8080 in your browser.

## Option 2: Development Mode (Recommended for Development)

For the best development experience with hot-reload:

### Terminal 1 - Backend Server
```bash
cd docs/ri-studio/server
go run main.go
```

### Terminal 2 - Frontend Dev Server
```bash
cd docs/ri-studio/web
npm install  # First time only
npm run dev
```

Then open http://localhost:3000 in your browser.

The frontend will automatically reload when you make changes, and API requests will be proxied to the backend.

## First Steps

1. **Try an Example**:
   - Click the "Load Example..." dropdown
   - Select "KServe" or "JobSet"
   - See how the RI is structured

2. **Validate**:
   - Click "Validate RI" to check your RI definition
   - Fix any errors shown in the results panel

3. **Extract**:
   - Add a sample CR in the left editor
   - Click "Extract" to see what data is extracted
   - Iterate on your RI definition until all fields are correctly extracted

## Common Commands

### Using Make
```bash
make help          # Show all available commands
make install       # Install frontend dependencies
make build         # Build frontend for production
make run-server    # Run the Go backend
```

### Manual Commands
```bash
# Backend
cd server
go run main.go

# Frontend - Development
cd web
npm run dev

# Frontend - Production Build
cd web
npm run build
```

## Troubleshooting

**Port already in use?**
```bash
# Backend (8080)
export PORT=8081
cd server && go run main.go

# Frontend (3000)
# Edit web/vite.config.ts and change the port
```

**Dependencies not found?**
```bash
# Go
cd server && go mod tidy

# Node
cd web && rm -rf node_modules && npm install
```

## What's Next?

- Read the full [README.md](README.md) for detailed documentation
- Check out examples in `../../examples/`
- Review the [Technical Guide](../Technical%20Guide.md) for RI best practices

## Need Help?

- Check server logs in Terminal 1
- Check browser console for frontend errors
- Verify both servers are running on expected ports
- Make sure you're in the correct directory when running commands

