# RI Studio - Quick Start Guide

Get started with RI Studio in just a few minutes! This is a **fully static application** - no backend server needed.

## Prerequisites

- **Go 1.24+**: `go version` (for building WASM)
- **Node.js 18+**: `node --version` (for building frontend)
- **npm**: `npm --version`

## Quick Start - Build & Preview

The fastest way to get started:

```bash
cd docs/ri-studio

# Build everything (WASM + Frontend)
make build-static

# Preview the static site
cd web && npm run preview
```

Then open **http://localhost:4173** in your browser.

That's it! The entire application runs in your browser with no backend server.

## Understanding the Build

When you run `make build-static`, it:

1. **Compiles Go to WASM** (~76MB binary)
   - Includes validation and extraction logic
   - Embeds all example YAML files
   
2. **Copies WASM Runtime** (`wasm_exec.js`)
   - Go's JavaScript bridge for WASM
   
3. **Builds React Frontend**
   - Optimized production build
   - Output in `web/dist/`

The result is a complete static site in `web/dist/` that you can:
- Open with any static file server
- Deploy to GitHub Pages, Netlify, Vercel, S3, etc.
- Even run offline after initial load

## Development Mode

For faster iteration during development:

```bash
cd docs/ri-studio/web
npm run dev
```

Then open **http://localhost:3000** in your browser.

**Note**: If you modify Go code, rebuild WASM:
```bash
cd docs/ri-studio
make build-wasm
# Refresh browser
```

## First Steps

1. **Try an Example**:
   - Click the "Load Example..." dropdown
   - Select "KServe" or "JobSet"
   - See how the RI is structured

2. **Validate**:
   - Click "Validate RI" to check your RI definition
   - Fix any errors shown in the results panel
   - **All validation runs in your browser!**

3. **Extract**:
   - Add a sample CR in the left editor
   - Click "Extract" to see what data is extracted
   - Iterate on your RI definition until all fields are correctly extracted
   - **Extraction runs locally using WASM!**

## Deploying to Production

### GitHub Pages

```bash
cd docs/ri-studio/web/dist
git init
git add -A
git commit -m 'deploy'
git push -f git@github.com:username/ri-studio.git main:gh-pages
```

### Netlify

```bash
cd docs/ri-studio
make build-static
netlify deploy --dir=web/dist --prod
```

### Vercel

```bash
cd docs/ri-studio
make build-static
vercel --prod web/dist
```

### Simple HTTP Server (Testing)

```bash
cd docs/ri-studio/web/dist
npx serve -p 8080
```

## Common Commands

```bash
# Build everything
make build-static

# Build only WASM (after Go changes)
make build-wasm

# Development mode
cd web && npm run dev

# Preview production build
cd web && npm run preview

# Clean build artifacts
make clean clean-wasm

# Show all commands
make help
```

## Architecture at a Glance

```
Browser
  ├── React Frontend (TypeScript)
  │   └── Monaco Editor for YAML editing
  └── WASM Module (Go)
      ├── Validation Logic (pkg/api/optimization/v1alpha1)
      ├── Extraction Logic (pkg/resource + pkg/query)
      └── Embedded Examples (docs/examples/*.yaml)
```

**No backend server. No API calls. Everything runs client-side.**

## Troubleshooting

### WASM not loading?

Check browser console for errors:
```javascript
// In browser console
wasmService.isReady()  // Should be true after ~1 second
```

### Build fails?

```bash
# Check Go version (needs 1.24+)
go version

# Update Go modules
cd wasm && go mod tidy

# Reinstall npm packages
cd web && rm -rf node_modules && npm install

# Try building WASM separately
make build-wasm
```

### Port already in use?

```bash
# Change preview port
cd web && npx vite preview --port 4174
```

### Examples not showing?

```bash
# Ensure examples are embedded
ls wasm/examples/

# If empty, copy them
cp -r ../../examples/* wasm/examples/

# Rebuild WASM
make build-wasm
```

## What's Different from Traditional Web Apps?

**Traditional**:
- React Frontend → HTTP API → Go Backend Server
- Requires server infrastructure
- Network latency on every action

**RI Studio (WASM)**:
- React Frontend → WASM (Go) in Browser
- Zero server infrastructure
- Instant validation/extraction

## What's Next?

- Read the full [README.md](README.md) for detailed documentation
- Check out examples in `../../examples/`
- Review the [Technical Guide](../Technical%20Guide.md) for RI best practices
- Try deploying to a static host!

## Need Help?

- **Browser Console**: Check for WASM errors
- **Network Tab**: Verify WASM is loading (should see `wasm.wasm` and `wasm_exec.js`)
- **WASM Status**: Run `wasmService.isReady()` in console
- **Build Logs**: Check terminal output during `make build-static`

Enjoy using RI Studio! 🚀
