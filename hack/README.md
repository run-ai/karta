# Hack Directory

This directory contains utility scripts, build tools, and development helpers for the Kai-bolt project.

## Contents

### UML Diagram Generation
- **`uml/`** - UML diagram generation tools with virtual environment isolation
  - `rid-uml-diagram.puml` - PlantUML source for RID architecture diagram
  - `generate_uml_diagram.py` - Automated script to generate PNG diagram
  - `README.md` - Detailed documentation for UML tools

## Usage

### Generate RID Architecture Diagram

```bash
# Run from repository root
python3 hack/uml/generate_uml_diagram.py
```

**Output**: `docs/design/rid-architecture.png`

The script handles:
- Virtual environment creation and management
- Dependency installation (`plantuml`, `six`, `httplib2`)
- Multiple generation methods (Python module, JAR fallback)
- Clear error reporting and manual alternatives

### Manual PlantUML Generation

If the automated script fails:

1. Visit [PlantUML Online](http://www.plantuml.com/plantuml/uml/)
2. Copy content from `hack/uml/rid-uml-diagram.puml`
3. Generate PNG and save as `docs/design/rid-architecture.png`

## Requirements

- **Python 3.x** for automation scripts and virtual environments
- **Java** (optional, for PlantUML JAR fallback)
- **Internet access** for dependency installation and PlantUML service

## Conventions

- **Input files**: Source code, templates, build configurations stay in hack/
- **Output files**: Generated content goes to appropriate `docs/` or project directories
- **Virtual environments**: Contained within tool directories for isolation
- **Temporary files**: Created and cleaned up automatically (e.g., downloaded JARs)
- **Scripts**: Self-contained with dependency management and error handling

## Directory Structure

```
hack/
├── README.md              # This file
└── uml/                   # UML generation tools
    ├── README.md          # UML-specific documentation
    ├── generate_uml_diagram.py
    ├── rid-uml-diagram.puml
    └── venv/              # Virtual environment (auto-created)
```

## Adding New Tools

When adding new utility scripts:

1. Create subdirectory under `hack/` for related tools
2. Use virtual environments for Python dependencies
3. Include README.md with usage instructions
4. Output generated files to appropriate project directories
5. Handle errors gracefully with clear messages 