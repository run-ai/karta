# Hack Directory

This directory contains utility scripts, build tools, and development helpers for the Kai-bolt project.

## Contents

### UML Diagram Generation
- **`uml/`** - UML diagram generation tools with virtual environment isolation
  - `rid-uml-diagram.puml` - PlantUML source for RID architecture diagram
  - `generate_uml_diagram.py` - Automated script to generate PNG diagram
  - `README.md` - Detailed documentation for UML tools

### Structure Definition Visualization
- **`structureDefinition/`** - Visual diagram generation for individual RID component structures
  - `generate_structure_definition.py` - Automated script to generate structure diagrams from RID YAML files
  - `requirements.txt` - Python dependencies for visualization
  - `venv/` - Virtual environment (auto-created)

## Usage

### Generate RID Architecture Diagram

```bash
# Run from repository root
python3 hack/uml/generate_uml_diagram.py
```

**Output**: `docs/design/rid-architecture.png`

This generates the overall RID API architecture diagram showing the new explicit structure with `rootComponent`, `childComponents`, and `referencedComponents`.

### Generate RID Structure Definition Diagrams

```bash
# Generate all RID structure diagrams
cd hack/structureDefinition
python3 generate_structure_definition.py --input-dir ../../docs/examples

# Generate specific framework diagrams
python3 generate_structure_definition.py --frameworks pytorch nimservice kserve

# Generate with custom input/output
python3 generate_structure_definition.py --input-dir /path/to/rids --output-path /path/to/output
```

**Output**: `docs/examples/*-structure-definition.png` (one per RID file)

This generates individual component hierarchy diagrams for each RID, showing:
- Root component (blue) with target Kubernetes resource
- Child components (white) owned by parent components
- Referenced components (gray) as external dependencies
- Ownership relationships (solid arrows)
- Reference relationships (dashed blue arrows)

The script handles:
- Virtual environment creation and dependency management
- Automatic discovery of RID YAML files
- Visual distinction between component types
- Support for the new explicit structure format
- Network graph layout optimization
- High-resolution PNG output

## New Explicit Structure Support

Both tools have been updated to support the new explicit RID structure:

- **Removed**: `topOwnerKind` field (target kind now in `rootComponent.kind`)
- **Removed**: `isReference` field (component type determined by section placement)
- **Added**: Explicit `rootComponent`, `childComponents`, `referencedComponents` sections
- **Enhanced**: Clear visual distinction between component types
- **Improved**: Self-documenting structure that eliminates ambiguity

## Requirements

- **Python 3.x** for automation scripts and virtual environments
- **Java** (optional, for PlantUML JAR fallback)
- **Internet access** for dependency installation and PlantUML service

## Directory Structure

```
hack/
├── README.md                    # This file
├── uml/                         # UML generation tools
│   ├── README.md                # UML-specific documentation
│   ├── generate_uml_diagram.py
│   ├── rid-uml-diagram.puml
│   └── venv/                    # Virtual environment (auto-created)
└── structureDefinition/         # Structure diagram generation
    ├── generate_structure_definition.py
    ├── requirements.txt
    └── venv/                    # Virtual environment (auto-created)
```

## Adding New Tools

When adding new utility scripts:

1. Create subdirectory under `hack/` for related tools
2. Use virtual environments for Python dependencies
3. Include README.md with usage instructions
4. Output generated files to appropriate project directories
5. Handle errors gracefully with clear messages
6. Support the current RID API structure and conventions 