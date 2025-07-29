# UML Diagram Generation

This directory contains tools for generating the RID architecture UML diagram.

## Contents

- **`rid-uml-diagram.puml`** - PlantUML source code for the RID architecture diagram
- **`generate_uml_diagram.py`** - Automated generation script with virtual environment isolation
- **`venv/`** - Python virtual environment (created automatically)

## Usage

### Generate Diagram

Run from the repository root:

```bash
python3 hack/uml/generate_uml_diagram.py
```

**Output**: `docs/design/rid-architecture.png`

### What the Script Does

1. **Virtual Environment Setup**: Creates isolated Python environment in `hack/uml/venv/`
2. **Dependency Installation**: Installs required packages (`plantuml`, `six`, `httplib2`)
3. **Diagram Generation**: Converts PlantUML source to PNG using web service
4. **Fallback Methods**: Downloads PlantUML JAR if Python method fails
5. **Clean Output**: Places generated diagram in `docs/design/` directory

### Manual Generation

If the automated script fails:

1. Visit [PlantUML Online](http://www.plantuml.com/plantuml/uml/)
2. Copy content from `hack/uml/rid-uml-diagram.puml`
3. Generate PNG and save as `docs/design/rid-architecture.png`

## Virtual Environment

The script creates and manages its own virtual environment to avoid system-wide package installation:

- **Location**: `hack/uml/venv/`
- **Packages**: plantuml, six, httplib2
- **Isolation**: No impact on system Python packages
- **Persistence**: Reused across multiple script runs

## Requirements

- **Python 3.x** - For virtual environment and script execution
- **Internet Access** - For package installation and PlantUML web service
- **Java** (optional) - For PlantUML JAR fallback method

## Troubleshooting

### Common Issues

1. **Permission Errors**: Ensure write access to `hack/uml/` and `docs/design/`
2. **Network Issues**: Check internet connection for package downloads
3. **Python Version**: Requires Python 3.3+ for venv module

### Clean Start

To reset the virtual environment:

```bash
rm -rf hack/uml/venv/
python3 hack/uml/generate_uml_diagram.py
```

## Architecture Notes

The PlantUML diagram shows:
- **Core RID Structure**: Main classes and relationships
- **Component Architecture**: Component definitions and properties
- **Optimization Instructions**: Available instruction types
- **Framework Examples**: Sample implementations

Keep the PlantUML source updated when RID structure changes. 