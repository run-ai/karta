# Current Iteration - ResourceInterpretationDefinition Examples

This directory contains the **current iteration** RID examples showcasing the latest RID structure with optimized instruction hierarchy and comprehensive framework support.

## Current Structure Features

### 1. Optimized Instruction Hierarchy
- **Removed**: `enforcement` field from individual instructions
- **Added**: `optimizationsInstructions` hierarchy in RID spec
- **Simplified**: Instructions are "hard-coded" fields (presence = required, absence = disabled)
- **Renamed**: Dropped "Instruction" suffix from instruction names

**Structure:**
```yaml
spec:
  optimizationsInstructions:
    gangScheduling:
      podGroups: [...]
    multiNodeNVLink:
      acceleratedComponents: [...]
    topologyAwareness:
      topologyGroups: [...]
```

### 2. Enhanced Reference Component Modeling
- **Replaced**: `isReference: bool` with `referenceDefinition` struct
- **Added**: `componentKeyPath` field for consistent referencing
- **Clarified**: Reference semantics (component referenced BY the main RID kind)

**Pattern:**
```yaml
- name: "nimcache-ref"
  specPath: ".spec"
  referenceDefinition:
    componentKeyPath: ".spec.storage.nimCache.name"
```

### 3. Research-Based Component Kinds
- **Methodology**: All component `kind` fields verified through framework research
- **Training Frameworks**: PyTorchJob/MPIJob components use `v1/Pod` (not `batch/v1/Job`)
- **Coordination Frameworks**: JobSet `replicatedjob` uses `batch/v1/Job`  
- **Stateful Frameworks**: LeaderWorkerSet components use `apps/v1/StatefulSet`
- **Serving Frameworks**: Components use `apps/v1/Deployment` patterns
- **Custom Frameworks**: NVIDIA Dynamo uses `DynamoComponentDeployment`

### 4. Comprehensive childKinds Implementation
- **Definition**: Custom compute resources created by main kind but not modeled as components
- **Rules**: Only unmodeled compute resources requiring operator traversal
- **Research**: Each framework analyzed for resource creation patterns
- **Examples**: Knative (Deployment), NIM (Job/Deployment), Dynamo (Deployment+LWS)

### 5. Structural Consistency
- **Root Components First**: All RIDs have main component as first in `structureDefinition`
- **Owner Hierarchies**: Complete `ownerName` relationships established
- **API Groups Verified**: All GVKs match actual framework implementations

## Available Examples

### AI/ML Training Frameworks
- **PyTorchJob** (`pytorch.yaml`) - Distributed PyTorch training
- **MPIJob** (`mpijob.yaml`) - MPI-based distributed training  

### Batch Processing
- **JobSet** (`jobset.yaml`) - Coordinated job execution
- **LeaderWorkerSet** (`lws.yaml`) - Leader-worker pattern

### Model Serving & Inference
- **NIMService** (`nimservice.yaml`) - NVIDIA NIM inference services
- **KServe** (`kserve.yaml`) - Serverless ML inference
- **Knative Serving** (`knative-serving.yaml`) - Serverless functions
- **NVIDIA Dynamo** (`dynamo.yaml`) - NVIDIA graph inference optimization

### Distributed Computing
- **RayCluster** (`raycluster.yaml`) - Ray distributed computing
- **Milvus** (`milvus.yaml`) - Vector database operations

### Model Management
- **NIMCache** (`nimcache.yaml`) - Model caching operations

## Instruction Types

### Gang Scheduling (`gangScheduling`)
Ensures coordinated pod scheduling for distributed workloads.
```yaml
gangScheduling:
  podGroups:
  - name: "training-gang"
    members:
    - componentDefinitionName: "master"
      componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
```

### Multi-Node NVLink (`multiNodeNVLink`)
Optimizes GPU communication for high-bandwidth workloads.
```yaml
multiNodeNVLink:
  acceleratedComponents:
  - componentDefinitionName: "worker"
    componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
    filter: '(.spec.template.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
```

### Topology Awareness (`topologyAwareness`)
Guides placement based on cluster topology for performance optimization.
```yaml
topologyAwareness:
  topologyGroups:
  - groupName: "training-cluster"
    preferredPlacement: "zone"
    members:
    - componentDefinitionName: "master"
      componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
```

## Component Referencing Pattern

For workloads that depend on external components (like NIMService → NIMCache):

1. **Main Component**: Defines the primary workload
2. **Reference Component**: Points to external dependency
3. **Dependency**: Expressed via `dependsOn` field
4. **Status Integration**: Reference components include status definitions for dependency checking

Example:
```yaml
structureDefinition:
- name: "nimservice"
  specPath: ".spec"
  dependsOn: ["nimcache-ref"]
  # ... status and other definitions

- name: "nimcache-ref"
  specPath: ".spec"
  referenceDefinition:
    componentKeyPath: ".spec.storage.nimCache.name"
  kind:
    group: "apps.nvidia.com"
    version: "v1alpha1"
    kind: "NIMCache"
  statusDefinition:
    # ... status mappings for dependency validation
```

## Validation Notes

- All condition types verified against actual framework implementations
- Component labels validated against framework standards
- JQ expressions tested for correctness
- Reference patterns follow Kubernetes conventions
- **Autoscaling configurations validated for serverless frameworks (Knative)**
  - **Knative Serving**: Uses bounds-only scaling (min/max), no direct replica control
  - **Traditional frameworks**: Use direct replica control with optional bounds

## Usage

These examples demonstrate the mature RID specification and can be used as:
- **Templates** for new framework integrations
- **Reference implementations** for instruction patterns
- **Validation examples** for RID structure correctness 