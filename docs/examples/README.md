# Kai-bolt RID Examples

This directory contains Resource Interpretation Definition (RID) examples for major AI/ML frameworks. These RIDs enable Kai-bolt to understand, monitor, and optimize distributed workloads on Kubernetes.

## Current Iteration Features

- **Added**: `optimizationInstructions` hierarchy in RID spec
- **Enhanced**: Component kinds match actual framework implementations  
- **Enhanced**: `childKinds` for compute resource traversal
- **Enhanced**: Root component ordering for clear hierarchy
- **Enhanced**: Elastic training support for autoscaling workloads

### Instruction Structure
```yaml
optimizationInstructions:
  gangScheduling: { ... }       # Present = required
  gpuInterconnect: { ... }      # Present = required  
  topologyAwareness: { ... }    # Present = required
  # Absent instructions = disabled
```

## Elastic Training Support

Different frameworks have **different scaling mechanisms** - RIDs model each framework according to its own semantics:

### PyTorchJob - Elastic Policy
**PyTorchJob** supports elastic training through worker-level elastic bounds:

```yaml
# Worker component with absolute JQ paths
structureDefinition:
  components:
  - name: "worker"
    specPath: ".spec.pytorchReplicaSpecs.Worker"    # Component context for primary operations
    scaleDefinition:
      replicasPath: ".spec.pytorchReplicaSpecs.Worker.replicas"  # Absolute path
      minReplicasPath: ".spec.elasticPolicy.minReplicas"        # Absolute path  
      maxReplicasPath: ".spec.elasticPolicy.maxReplicas"        # Absolute path

  # Master component (fixed)
  - name: "master"
    scaleDefinition:
      replicasPath: ".spec.pytorchReplicaSpecs.Master.replicas"  # Absolute path
```

**JQ Path Resolution in RIDs:**
- **All JQ paths are absolute** from the Kubernetes resource root
- **`specPath`** sets the component's primary context (for component-specific operations)
- **Scale/Status/Reference paths** are independent absolute JQ expressions

```yaml
# Kubernetes Resource Structure:
apiVersion: kubeflow.org/v1
kind: PyTorchJob
spec:                              # ← Root for all JQ paths
  elasticPolicy:
    minReplicas: 1                 # ← .spec.elasticPolicy.minReplicas
    maxReplicas: 4                 # ← .spec.elasticPolicy.maxReplicas
  pytorchReplicaSpecs:
    Worker:
      replicas: 2                  # ← .spec.pytorchReplicaSpecs.Worker.replicas
```

## JQ Path Architecture

**RID component definitions use two categories of JQ expressions with different evaluation contexts:**

### Component Definition Paths (Absolute from Main Resource)
All paths in component definitions use **absolute JQ expressions** from the main RID resource root:

```yaml
- name: "worker"
  specPath: ".spec.pytorchReplicaSpecs.Worker"              # Component context
  scaleDefinition:
    replicasPath: ".spec.pytorchReplicaSpecs.Worker.replicas"  # Absolute from PyTorchJob
    minReplicasPath: ".spec.elasticPolicy.minReplicas"        # Absolute from PyTorchJob
    maxReplicasPath: ".spec.elasticPolicy.maxReplicas"        # Absolute from PyTorchJob
  statusDefinition:
    conditionsPath: ".status.conditions"                     # Absolute from PyTorchJob
```

**Key Rules:**
- **All component paths start with leading `.`** (e.g., `.spec`, `.status`)
- **Paths are evaluated against the main RID resource** (PyTorchJob, JobSet, etc.)
- **For reference components**: Paths are absolute from the **referenced resource**

### Instruction Filter Paths (Relative to Component Resource)
Instruction filters use **relative paths from the component's actual resource type**:

```yaml
optimizationInstructions:
  multiNodeNVLink:
    acceleratedComponents:
    - componentDefinitionName: "worker"              # Component type: Pod
      filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
      # ↑ Evaluated against Pod resource, not PyTorchJob
```

**Key Rules:**
- **Filter paths are relative** to the component's `kind` resource type
- **PyTorchJob Pods**: Use `.spec.containers[0]...`
- **JobSet Jobs**: Use `.spec.template.spec.containers[0]...`
- **StatefulSets**: Use `.spec.template.spec.containers[0]...`

### Path Architecture Examples

#### **Fixed Scaling (MPIJob)**
```yaml
# Components with fixed scaling only
- name: "launcher"
  specPath: ".spec.mpiReplicaSpecs.Launcher"
  scaleDefinition:
    replicasPath: ".spec.mpiReplicaSpecs.Launcher.replicas"  # Absolute

- name: "worker"
  specPath: ".spec.mpiReplicaSpecs.Worker"
  scaleDefinition:
    replicasPath: ".spec.mpiReplicaSpecs.Worker.replicas"    # Absolute
```

#### **Array-Based Scaling (JobSet)**
```yaml
# Array iteration with absolute paths
- name: "replicatedjob"
  specPath: ".spec.replicatedJobs[]"                        # Array pattern
  scaleDefinition:
    replicasPath: ".spec.replicatedJobs[].replicas"         # Absolute with array
```

#### **Annotation-Based Scaling (Knative)**
```yaml
# Service-level autoscaling (different mechanism)
- name: "service"
  specPath: ".spec"
  scaleDefinition:
    minReplicasPath: '.spec.template.metadata.annotations["autoscaling.knative.dev/min-scale"]'
    maxReplicasPath: '.spec.template.metadata.annotations["autoscaling.knative.dev/max-scale"]'
```

### Reference Component Paths

For workloads that depend on external components (like NIMService → NIMCache):

```yaml
# Referencing component owns the relationship
- name: "nimservice"
  specPath: ".spec"                                    # NIMService resource context
  references:
    - componentName: "nimcache-ref"
      componentKeyPath: ".spec.storage.nimCache.name"  # Absolute from NIMService (main RID)
  dependsOn: ["nimcache-ref"]

# Referenced component marked as external  
- name: "nimcache-ref"
  specPath: ".spec"                                    # NIMCache resource context
  isReference: true
  statusDefinition:
    conditionsPath: ".status.conditions"              # Absolute from NIMCache resource
```

**Key Rules:**
- **Referencing component `references[].componentKeyPath`**: Absolute from **main RID resource**
- **Referenced component `specPath`, `statusDefinition` paths**: Absolute from **referenced resource**
- **`isReference: true`**: Marks component as external dependency

### Why This Architecture?

1. **Predictable Path Resolution**: Always know which resource the path targets
2. **PyTorchJob Elastic Policy Reachable**: Worker components can access job-level configuration
3. **Array Support**: JobSet/RayCluster can model all replicated jobs/worker groups
4. **Reference Components Work**: Clear separation between main and referenced resources
5. **Instruction Filters Stay Simple**: Relative paths match intuitive component structure

### MPIJob - Fixed Scaling Only
**MPIJob** uses fixed replica counts without elastic capabilities:

```yaml
# Components with fixed scaling only
structureDefinition:
  components:
  - name: "launcher"
    scaleDefinition:
      replicasPath: ".spec.mpiReplicaSpecs.Launcher.replicas"  # Fixed count only

  - name: "worker"
    scaleDefinition:
      replicasPath: ".spec.mpiReplicaSpecs.Worker.replicas"    # Fixed count only
```

### Knative - Annotation-Based Autoscaling
**Knative** uses service-level autoscaling annotations:

```yaml
# Service-level autoscaling (different mechanism)
structureDefinition:
  components:
  - name: "service"
    scaleDefinition:
      minReplicasPath: '.spec.template.metadata.annotations["autoscaling.knative.dev/min-scale"]'
      maxReplicasPath: '.spec.template.metadata.annotations["autoscaling.knative.dev/max-scale"]'
```

### Framework-Specific Benefits
- **PyTorchJob**: Fault tolerance, spot instance support, TorchElastic integration
- **MPIJob**: Predictable resource allocation, fixed coordination patterns  
- **Knative**: Traffic-based scaling, serverless efficiency

## Current Structure Features

### 1. Optimized Instruction Hierarchy
- **Removed**: `enforcement` field from individual instructions
- **Added**: `optimizationInstructions` hierarchy in RID spec
- **Simplified**: Instructions are "hard-coded" fields (presence = required, absence = disabled)
- **Renamed**: Dropped "Instruction" suffix from instruction names

**Structure:**
```yaml
spec:
  topOwnerKind:
    group: kubeflow.org
    version: v1
    kind: PyTorchJob
  structureDefinition:
    components:
    - name: "pytorchjob"
      specPath: ".spec"
      kind: { group: "kubeflow.org", version: "v1", kind: "PyTorchJob" }
    - name: "master"
      ownerName: "pytorchjob"
      scaleDefinition:
        replicasPath: ".spec.pytorchReplicaSpecs.Master.replicas"
    - name: "worker"
      ownerName: "pytorchjob"
      scaleDefinition:
        replicasPath: ".spec.pytorchReplicaSpecs.Worker.replicas"
        minReplicasPath: ".spec.elasticPolicy.minReplicas"
        maxReplicasPath: ".spec.elasticPolicy.maxReplicas"
  optimizationInstructions:
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

### 4. Comprehensive additionalChildKinds Implementation
- **Definition**: Custom compute resources created by main kind but not modeled as components
- **Rules**: Only unmodeled compute resources requiring operator traversal
- **Research**: Each framework analyzed for resource creation patterns
- **Examples**: Knative (Deployment), NIM (Job/Deployment), Dynamo (Deployment+LWS)

```yaml
# Knative example with child resources
structureDefinition:
  additionalChildKinds:
  - group: apps
    version: v1
    kind: Deployment
  components:
  - name: "service"
    # ... component definition
```

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

## Optimization Instructions

The examples demonstrate three types of optimization instructions:

### Gang Scheduling (`gangScheduling`)
Ensures related pods are scheduled together as a unit, preventing partial deployments that could lead to deadlocks or resource waste.

### GPU Interconnect (`gpuInterconnect`)  
Optimizes workloads requiring high-bandwidth GPU interconnects (NVLink, InfiniBand, etc.) for multi-node GPU communication and distributed processing.

### Topology Awareness (`topologyAwareness`)
Guides pod placement based on cluster topology to minimize network latency and optimize resource locality.

## Component Referencing Pattern

For workloads that depend on external components (like NIMService → NIMCache):

1. **Referencing Component**: Owns the reference relationship via `references` list
2. **Referenced Component**: Marked as external with `isReference: true`
3. **Dependency**: Expressed via `dependsOn` field
4. **Status Integration**: Referenced components include status definitions for dependency checking

Example:
```yaml
structureDefinition:
  components:
  - name: "nimservice"
    specPath: ".spec"
    references:
      - componentName: "nimcache-ref"
        componentKeyPath: ".spec.storage.nimCache.name"
    dependsOn: ["nimcache-ref"]
    # ... status and other definitions

  - name: "nimcache-ref"
    specPath: ".spec"
    isReference: true
    kind:
      group: "apps.nvidia.com"
      version: "v1alpha1"
      kind: "NIMCache"
    statusDefinition:
      # ... status mappings for dependency validation
```

**Critical Rules**:
- Referencing components use `references` list to specify dependencies  
- `componentKeyPath` is evaluated against the referencing component's resource
- Referenced components are marked with `isReference: true`
- Referenced components must have `statusDefinition` for monitoring
- Creates orchestration order: cache → service
- Prevents invalid startup sequences

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