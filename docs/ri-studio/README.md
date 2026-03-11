# RI Studio - Resource Interface Authoring Tool

RI Studio is a **fully static web application** for creating and testing Resource Interface (RI) definitions for Kubernetes Custom Resources. It provides a visual interface with split-pane YAML editors, real-time validation, and extraction preview capabilities - all running entirely in your browser with no backend server required!

## Features

- **Dual YAML Editors**: Side-by-side editors for Custom Resource (CR) and Resource Interface (RI) definitions
- **Real-time Validation**: Validate RI definitions using the same validation logic as the production system
- **Extraction Preview**: See exactly what data the utilities can extract from your CR using your RI definition
- **Example Templates**: Pre-loaded examples for common frameworks (KServe, JobSet, PyTorchJob, etc.)
- **Monaco Editor**: Full-featured code editor with syntax highlighting, line numbers, and auto-formatting
- **100% Static**: No backend server needed - deploy anywhere that serves static files
- **Offline Capable**: Works offline after initial load

## Architecture

### WebAssembly (WASM) Core
- **Location**: `docs/ri-studio/wasm/`
- **Stack**: Go compiled to WebAssembly
- **Features**:
  - RI validation using production validation logic
  - CR data extraction using `pkg/resource` and `pkg/query`
  - Embedded example templates
  - ~76MB WASM binary (includes Go runtime + k8s libraries)

### Frontend (React + TypeScript)
- **Location**: `docs/ri-studio/web/`
- **Stack**: React 18, TypeScript, Vite, TailwindCSS
- **Features**:
  - Monaco Editor integration
  - Responsive split-pane layout
  - WASM loader and type-safe bindings
  - Real-time results display

### Legacy Server (Optional)
- **Location**: `docs/ri-studio/server/`
- **Note**: The Go HTTP server is kept for reference but is no longer needed for the static WASM build

## Getting Started

### Prerequisites

- **For Development**: Go 1.24+, Node.js 18+, npm
- **For Deployment**: Any static file server (or just open the files!)

### Quick Start - Build Static Site

```bash
cd docs/ri-studio
make build-static
```

This will:
1. Compile Go code to WASM (~76MB binary)
2. Copy `wasm_exec.js` from Go SDK
3. Install frontend dependencies
4. Build optimized React app

The complete static site will be in `web/dist/` - ready to deploy!

### Testing Locally

After building, preview the static site:

```bash
cd docs/ri-studio/web
npm run preview
```

Then open http://localhost:4173 in your browser.

### Development Mode

For faster iteration during development:

```bash
cd docs/ri-studio/web
npm run dev
```

Then open http://localhost:3000 in your browser.

**Note**: In dev mode, you'll need to rebuild WASM if you change Go code:
```bash
cd docs/ri-studio
make build-wasm
```

## Usage

### Basic Workflow

1. **Load an Example** (optional):
   - Click the "Load Example..." dropdown in the toolbar
   - Select an example like "KServe" or "JobSet"
   - The RI definition will be loaded into the right editor

2. **Edit your Custom Resource**:
   - Use the left editor to define or paste your CR YAML
   - This is the actual Kubernetes resource you want to work with

3. **Edit your Resource Interface**:
   - Use the right editor to define or modify your RI YAML
   - Define how to extract pod specs, status, scaling info, etc.

4. **Validate the RI**:
   - Click "Validate RI" button
   - See validation results in the bottom panel
   - Fix any errors shown

5. **Test Extraction**:
   - Click "Extract" button
   - See what data was successfully extracted from your CR
   - Verify all components and fields are correctly extracted

6. **Iterate**:
   - Adjust JQ paths in your RI definition
   - Re-test extraction until everything works correctly

### Understanding Results

The results panel shows:

- **Validation Results**: 
  - ✓ Success or list of validation errors
  - Errors include details about what's wrong with the RI definition

- **Extraction Results**:
  - **Components**: Each component defined in your RI
  - **Instance IDs**: Array of instance identifiers (if applicable)
  - **Pod Specs**: Extracted PodTemplateSpec, PodSpec, or FragmentedPodSpec
  - **Scale Information**: Replicas, min/max values
  - **Errors**: Any extraction errors with context

## Deployment Options

The static site in `web/dist/` can be deployed to any static hosting platform:

### GitHub Pages

```bash
# After building
cd web/dist
git init
git add -A
git commit -m 'deploy'
git push -f git@github.com:your-username/ri-studio.git main:gh-pages
```

### Netlify

```bash
# Install Netlify CLI
npm install -g netlify-cli

# Deploy
cd web/dist
netlify deploy --prod
```

### Vercel

```bash
# Install Vercel CLI
npm install -g vercel

# Deploy
cd web/dist
vercel --prod
```

### AWS S3 + CloudFront

```bash
# Upload to S3
aws s3 sync web/dist/ s3://your-bucket-name/ --delete

# Configure CloudFront to serve index.html as default
```

### Any Web Server (nginx, Apache, etc.)

Simply copy the contents of `web/dist/` to your web server's document root.

**Important**: Ensure your web server serves `index.html` for all routes (SPA routing).

## Project Structure

```
docs/ri-studio/
├── wasm/                   # WASM core (NEW)
│   ├── main.go            # WASM entry point with exported functions
│   ├── go.mod             # WASM module dependencies
│   └── examples/          # Embedded example YAMLs
│
├── web/                   # React frontend
│   ├── public/            # Static assets (built files go here)
│   │   ├── wasm.wasm      # Generated WASM binary
│   │   └── wasm_exec.js   # Go WASM runtime
│   ├── src/
│   │   ├── components/    # React components
│   │   │   ├── EditorPanel.tsx
│   │   │   ├── Toolbar.tsx
│   │   │   ├── ResultsPanel.tsx
│   │   │   ├── ComponentResult.tsx
│   │   │   └── ErrorDisplay.tsx
│   │   ├── services/      # Service layer
│   │   │   ├── wasm.ts    # WASM loader & bindings (NEW)
│   │   │   └── api.ts     # API wrapper (uses WASM)
│   │   ├── types/         # TypeScript types
│   │   │   └── index.ts
│   │   ├── App.tsx        # Main app component
│   │   ├── main.tsx       # React entry point
│   │   └── index.css      # Global styles
│   ├── dist/              # Production build output
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── tailwind.config.js
│
├── server/                # Legacy Go server (OPTIONAL - for reference)
│   └── ...
│
└── Makefile              # Build automation
```

## WASM Functions

The WASM module exposes these JavaScript functions:

- `validateRI(riYAML: string)` → Validates an RI definition
- `extractData(crYAML: string, riYAML: string)` → Extracts data from a CR
- `listExamples()` → Lists embedded example templates
- `getExample(name: string)` → Gets a specific example template

All functions are wrapped with TypeScript type-safe bindings in `web/src/services/wasm.ts`.

## Development Tips

### WASM Development

- Modify code in `wasm/main.go`
- Run `make build-wasm` to recompile
- Refresh browser to load new WASM binary
- Check browser console for WASM errors

### Frontend Development

- Vite provides hot module replacement (HMR)
- TailwindCSS for styling with utility classes
- Monaco Editor for advanced code editing features
- TypeScript for type safety
- WASM loads automatically on app startup

### Adding New Examples

1. Add YAML file to `docs/examples/`
2. Copy examples to `wasm/examples/`: `cp -r docs/examples/* wasm/examples/`
3. Rebuild WASM: `make build-wasm`
4. The example will be embedded and appear in the dropdown

## Build Commands

```bash
# Full static build (WASM + Frontend)
make build-static

# Build only WASM
make build-wasm

# Install frontend dependencies
make install

# Clean build artifacts
make clean clean-wasm

# Show all commands
make help
```

## Troubleshooting

### WASM won't load
- Check browser console for errors
- Verify `wasm.wasm` and `wasm_exec.js` exist in `web/public/`
- Ensure files are served with correct MIME types
- WASM requires HTTP(S) - won't work with `file://` protocol

### Build fails
- **Go version**: Ensure Go 1.24+ is installed
- **Go modules**: Run `cd wasm && go mod tidy`
- **Node modules**: Run `cd web && npm install`
- **Disk space**: WASM binary is ~76MB

### WASM functions not available
- Wait for WASM to initialize (check browser console for "WASM module initialized")
- WASM initialization happens automatically on page load
- Check `wasmService.isReady()` in console

### Examples don't load
- Verify examples are copied to `wasm/examples/`
- Rebuild WASM to embed new examples
- Check browser console for errors

## Performance Considerations

- **Initial Load**: ~76MB WASM download + instantiation (~1-2 seconds on fast connections)
- **Execution Speed**: Near-native Go performance for validation/extraction
- **Memory Usage**: WASM runs in isolated memory (typical usage: 50-100MB)
- **Offline**: Works offline after initial load (WASM + examples are cached)

## Browser Compatibility

- **Chrome**: ✅ Full support
- **Firefox**: ✅ Full support
- **Safari**: ✅ Full support (macOS 11.3+, iOS 14.5+)
- **Edge**: ✅ Full support

WebAssembly is supported in all modern browsers (2017+).

## Contributing

When contributing to RI Studio:

1. **WASM changes**: Go code in `docs/ri-studio/wasm/`
2. **Frontend changes**: TypeScript/React in `docs/ri-studio/web/`
3. Keep types in sync between Go structs and TypeScript interfaces
4. Test both validation and extraction with various RI examples
5. Rebuild WASM after Go changes: `make build-wasm`
6. Follow existing code style and patterns

## Why WASM?

**Benefits**:
- ✅ No backend infrastructure needed
- ✅ Deploy anywhere (GitHub Pages, S3, CDN, etc.)
- ✅ Reuses production Go code (same validation/extraction logic)
- ✅ Works offline after initial load
- ✅ Fast execution (near-native performance)
- ✅ Type-safe across Go and TypeScript

**Trade-offs**:
- ⚠️ Large initial download (~76MB)
- ⚠️ Limited debugging in browser
- ⚠️ Requires HTTP(S) (not file://)

## License

This tool is part of the kai-bolt project and follows the same license.




