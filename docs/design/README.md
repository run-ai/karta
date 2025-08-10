# Kai-bolt Design Documentation

This folder contains comprehensive design documentation for Resource Interpretation Definitions (RIDs) and the Kai-bolt optimization system. These documents enable immediate productive collaboration on AI/ML workload optimization.

## RID Architecture Diagram

A comprehensive UML diagram showing the complete RID structure is available:

- **Source**: [`hack/uml/rid-uml-diagram.puml`](../../hack/uml/rid-uml-diagram.puml) - PlantUML source code
- **Generated**: [`rid-architecture.png`](rid-architecture.png) - Visual diagram (generated from source)
- **Generator**: [`hack/uml/generate_uml_diagram.py`](../../hack/uml/generate_uml_diagram.py) - Automated generation script

### Generating the Diagram

To generate or update the architecture diagram:

```bash
# Run from repository root
python3 hack/uml/generate_uml_diagram.py
```

The script will:
- Create isolated virtual environment in `hack/uml/venv/`
- Install required Python packages (`plantuml`, `six`, `httplib2`)
- Generate PNG from PlantUML source in `hack/uml/` directory
- Output diagram to `docs/design/rid-architecture.png`
- Provide fallback methods if primary approach fails (JAR download)
- Give manual alternatives if automation fails

**Manual alternative**: Visit [PlantUML Online](http://www.plantuml.com/plantuml/uml/) and paste the content from `hack/uml/rid-uml-diagram.puml`.

## 📖 Documentation Overview

### For New Collaborators
**Start here for quick onboarding:**

1. **[`kai-bolt-overview.md`](kai-bolt-overview.md)** - What Kai-bolt is and how RIDs work
2. **[`rid-principles.md`](rid-principles.md)** - Non-negotiable architectural rules and design principles
3. **[`framework-patterns.md`](framework-patterns.md)** - How different AI/ML frameworks should be modeled
4. **[`validation-checklist.md`](validation-checklist.md)** - Quality assurance checklist for every RID

### For Ongoing Development
**Reference during active development:**

5. **[`architectural-decisions.md`](architectural-decisions.md)** - Key design decisions and their rationale
6. **[`common-mistakes.md`](common-mistakes.md)** - Mistakes to avoid based on real experience
7. **[`iteration-process.md`](iteration-process.md)** - Systematic approach to RID development

## Usage Scenarios

### **Starting Fresh on Kai-bolt**
**Read**: `kai-bolt-overview.md` → `rid-principles.md` → `framework-patterns.md`  
**Purpose**: Understand core concepts and design patterns

### **Adding a New Framework**
**Read**: `framework-patterns.md` → `validation-checklist.md` → `common-mistakes.md`  
**Purpose**: Model framework correctly and avoid common issues

### **Troubleshooting RID Issues**
**Read**: `common-mistakes.md` → `architectural-decisions.md` → `validation-checklist.md`  
**Purpose**: Debug problems and understand architectural constraints

### **Understanding Design Decisions**
**Read**: `architectural-decisions.md` → `rid-principles.md`  
**Purpose**: Learn why specific patterns were chosen

### **Framework Research and Analysis**
**Read**: `framework-patterns.md` → Examples in `docs/examples/`  
**Purpose**: Understand how different frameworks are modeled

## Critical Knowledge ("Pins")

These are the **most important rules** that cannot be broken:

### **1. JQ Path Architecture**
- **Component Definition Paths**: Always absolute from main RID resource root (`.spec.pytorchReplicaSpecs.Worker.replicas`)
- **Instruction Filter Paths**: Always relative to component's resource type (`.spec.containers[0].resources`)

### **2. Reference Architecture**  
- **Referencing component**: Owns the relationship via `references` list (in rootComponent or childComponents)
- **Referenced component**: Placed in `referencedComponents` section
- **Path evaluation**: All paths within a component evaluate against same resource

### **3. Status Definitions Must Be Research-Based**
All condition types must match actual framework APIs - never use generic assumptions.

### **4. Component Kinds Must Match Reality**
Every component `kind` must match the actual Kubernetes resource created by the framework.

## 📁 Related Directories

### Examples and Reference Implementations
- **`docs/examples/`** - Current RID implementations for major frameworks (latest validated versions)
- **`docs/examples/README.md`** - Current iteration features and framework coverage
- **`docs/examples/iteration-01/`** - Archived: Historical first iteration for reference
- **`docs/examples/iteration-02/`** - Archived: Previous iteration showing evolution
- **`pkg/api/v1alpha1/`** - Go struct definitions for RID schema

### Framework Technical Documentation
- **`docs/design/frameworks/`** - CRD and controller analysis for each framework
- **`docs/design/frameworks/README.md`** - Overview of framework documentation standards
- Individual framework docs: PyTorchJob, NIMService, JobSet, Knative Serving, etc.
- Source code verified architecture patterns and resource creation behaviors

## 🔄 Document Relationships

```
kai-bolt-overview.md (foundation)
    ↓
rid-principles.md (rules)
    ↓
framework-patterns.md (patterns)
    ↓
iteration-process.md (methodology)
    ↓
validation-checklist.md (quality)
    ↑
common-mistakes.md (prevention)
    ↑
architectural-decisions.md (rationale)
```

## Framework Coverage

The design documents cover patterns for:

### **Distributed Training** 
- PyTorch, MPI, TensorFlow
- Gang scheduling (required), Zone topology, Multi-GPU coordination

### **Model Inference**
- KServe, Knative Serving, NVIDIA NIM
- Gang scheduling (preferred), Node topology, Auto-scaling

### **Batch Coordination**
- JobSet, LeaderWorkerSet
- Gang scheduling (required), Zone topology, Resource efficiency

### **Distributed Computing**
- Ray, Spark
- Cluster patterns, Dynamic scaling, Head-worker architectures

### **Multi-CRD Frameworks**
- NVIDIA NIM (NIMCache + NIMService)
- Separate optimization domains, Explicit dependencies

## Quick Reference

### **Design Validation Commands**
```bash
# Check for common mistakes
grep -r 'kind: "' docs/examples/          # String kind fields
grep -r 'replica-type' docs/examples/     # Wrong componentKeyPath
grep -r '"nvidia.com/gpu"] > 0' docs/examples/ # Unsafe filters
```

### **Framework Research Pattern**
- Study framework documentation and CRD definitions
- Research status conditions in source code repositories
- Review community examples and operator guides
- Cross-reference multiple sources for accuracy

### **RID Structure Template**
```yaml
apiVersion: kai-bolt.runai.ai/v1alpha1
kind: ResourceInterpretationDefinition
metadata:
  name: "[framework]"
spec:
  structureDefinition:
    rootComponent:
      name: "[main-component]"
      kind: { group, version, kind }
      statusDefinition: { conditionsDefinition, statusMappings }
    
    childComponents:              # Optional - if framework has child resources
    - name: "[child-component]"
      ownerName: "[main-component]"
      kind: { group, version, kind }
      childSpecDefinition: { ... }
    
    referencedComponents:         # Optional - if framework has dependencies
    - name: "[reference-name]"
      kind: { group, version, kind }
      statusDefinition: { ... }
    
    additionalChildKinds:         # Optional - unmodeled child resources
    - group: apps
      version: v1
      kind: Deployment
      
  optimizationInstructions:
    gangScheduling:
      podGroups: [...]
    gpuInterconnect:
      acceleratedComponents: [...]
    topologyAwareness:
      topologyGroups: [...]
```

## 🎓 Learning Path

### **Week 1: Foundation**
- Complete reading of overview, principles, and patterns
- Study 2-3 existing RID examples
- Practice with validation checklist

### **Week 2: Application**
- Follow iteration process for a simple framework
- Create functional RID with peer review
- Learn from mistakes using common-mistakes guide

### **Week 3: Mastery**
- Work on complex multi-CRD framework
- Contribute to architectural decisions
- Mentor new contributors

## Maintenance

### **Keeping Documents Current**
- Update when new framework patterns emerge
- Revise when architectural decisions change
- Add new mistakes as they're discovered
- Expand validation checklist based on experience

### **Version Alignment**
- Documents should reflect current RID API version
- Examples should use latest established patterns
- Migration guides for breaking changes

## 💡 Contributing

When contributing to these design documents:

1. **Follow established structure** - Keep consistency across documents
2. **Include examples** - Concrete examples are more valuable than abstract rules
3. **Reference real experience** - Base recommendations on actual development work
4. **Update cross-references** - Maintain document relationships
5. **Test recommendations** - Ensure guidance actually works in practice

## 🎪 Success Indicators

You'll know these documents are working when:

- New contributors can create quality RIDs quickly
- Common mistakes become rare
- Framework experts can validate RIDs easily
- Development time decreases while quality increases
- Architectural consistency improves across all RIDs

---

*These documents represent distilled knowledge from intensive Kai-bolt RID development. They capture not just what to do, but why, how, and what to avoid - enabling immediate productive collaboration on AI/ML workload optimization.* 