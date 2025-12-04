# RI Studio - Resource Interface Authoring Tool

RI Studio is a web-based application for creating and testing Resource Interface (RI) definitions for Kubernetes Custom Resources. It provides a visual interface with split-pane YAML editors, real-time validation, and extraction preview capabilities.

## Features

- **Dual YAML Editors**: Side-by-side editors for Custom Resource (CR) and Resource Interface (RI) definitions
- **Real-time Validation**: Validate RI definitions using the same validation logic as the production system
- **Extraction Preview**: See exactly what data the utilities can extract from your CR using your RI definition
- **Example Templates**: Pre-loaded examples for common frameworks (KServe, JobSet, PyTorchJob, etc.)
- **Monaco Editor**: Full-featured code editor with syntax highlighting, line numbers, and auto-formatting

## Architecture

### Backend (Go)
- **Location**: `docs/ri-studio/server/`
- **Stack**: Go HTTP server
- **Features**:
  - REST API for validation and extraction
  - Integrates with existing `pkg/` utilities
  - Serves static frontend files

### Frontend (React + TypeScript)
- **Location**: `docs/ri-studio/web/`
- **Stack**: React 18, TypeScript, Vite, TailwindCSS
- **Features**:
  - Monaco Editor integration
  - Responsive split-pane layout
  - Real-time results display

## Getting Started

### Prerequisites

- Go 1.24+ (for backend)
- Node.js 18+ and npm (for frontend)

### Installation

1. **Install frontend dependencies**:
   ```bash
   cd docs/ri-studio/web
   npm install
   ```

### Development Mode

Run both the backend and frontend servers:

1. **Terminal 1 - Start the Go backend**:
   ```bash
   # From the project root
   cd docs/ri-studio/server
   go run main.go
   ```
   The API server will start on http://localhost:8080

2. **Terminal 2 - Start the React dev server**:
   ```bash
   # From the project root
   cd docs/ri-studio/web
   npm run dev
   ```
   The frontend will start on http://localhost:3000

3. **Open your browser** to http://localhost:3000

The Vite dev server will proxy API requests to the Go backend automatically.

### Production Build

1. **Build the frontend**:
   ```bash
   cd docs/ri-studio/web
   npm run build
   ```
   This creates optimized static files in `web/dist/`

2. **Run the server**:
   ```bash
   cd docs/ri-studio/server
   go run main.go
   ```
   
3. **Open your browser** to http://localhost:8080

The Go server will serve the React app and handle API requests.

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

## Project Structure

```
docs/ri-studio/
├── server/                 # Go backend
│   ├── main.go            # HTTP server entry point
│   ├── handlers/          # API request handlers
│   │   └── api.go
│   ├── service/           # Business logic
│   │   ├── validator.go   # RI validation
│   │   └── extractor.go   # Data extraction
│   └── models/            # API types
│       └── api.go
│
└── web/                   # React frontend
    ├── src/
    │   ├── components/    # React components
    │   │   ├── EditorPanel.tsx
    │   │   ├── Toolbar.tsx
    │   │   ├── ResultsPanel.tsx
    │   │   ├── ComponentResult.tsx
    │   │   └── ErrorDisplay.tsx
    │   ├── services/      # API client
    │   │   └── api.ts
    │   ├── types/         # TypeScript types
    │   │   └── index.ts
    │   ├── App.tsx        # Main app component
    │   ├── main.tsx       # React entry point
    │   └── index.css      # Global styles
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    └── tailwind.config.js
```

## API Endpoints

### POST /api/validate
Validates an RI definition.

**Request**:
```json
{
  "ri": "apiVersion: optimization.nvidia.com/v1alpha1\nkind: ResourceInterface\n..."
}
```

**Response**:
```json
{
  "valid": true,
  "errors": []
}
```

### POST /api/extract
Extracts information from a CR using an RI definition.

**Request**:
```json
{
  "cr": "apiVersion: example.com/v1\nkind: Example\n...",
  "ri": "apiVersion: optimization.nvidia.com/v1alpha1\nkind: ResourceInterface\n..."
}
```

**Response**:
```json
{
  "success": true,
  "components": [
    {
      "name": "example",
      "kind": { "group": "example.com", "version": "v1", "kind": "Example" },
      "instanceIds": [""],
      "podTemplateSpec": {...}
    }
  ]
}
```

### GET /api/examples
Lists available example templates.

### GET /api/examples/:name
Gets a specific example template.

## Environment Variables

### Backend

- `PORT`: Server port (default: 8080)
- `EXAMPLES_PATH`: Path to examples directory (auto-detected)
- `WEB_DIST_PATH`: Path to frontend build directory (auto-detected)

## Development Tips

### Backend Development

- The server automatically reloads when using `go run`
- API responses are JSON with CORS enabled
- Errors are returned in a consistent format

### Frontend Development

- Vite provides hot module replacement (HMR)
- TailwindCSS for styling with utility classes
- Monaco Editor for advanced code editing features
- TypeScript for type safety

### Adding New Examples

1. Create a YAML file in `docs/examples/`
2. The example will automatically appear in the dropdown
3. File name becomes the example identifier

## Troubleshooting

### Backend won't start
- Ensure you're in the correct directory: `docs/ri-studio/server/`
- Check that Go 1.24+ is installed: `go version`
- Verify all Go dependencies are available

### Frontend won't start
- Ensure dependencies are installed: `npm install`
- Check Node.js version: `node --version` (needs 18+)
- Clear node_modules and reinstall if needed

### API requests fail
- Verify the backend is running on port 8080
- Check browser console for CORS errors
- Ensure the Vite proxy is configured correctly

### Examples don't load
- Verify the `docs/examples/` directory exists
- Check that YAML files are valid
- Look at server logs for error messages

## Contributing

When contributing to RI Studio:

1. Backend changes go in `docs/ri-studio/server/`
2. Frontend changes go in `docs/ri-studio/web/`
3. Keep the API contract in sync between backend and frontend types
4. Test both validation and extraction with various RI examples
5. Follow existing code style and patterns

## License

This tool is part of the kai-bolt project and follows the same license.




