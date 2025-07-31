# Kai-bolt Overview

## What is Kai-bolt?

Kai-bolt is a Kubernetes optimization system that intelligently manages AI/ML workloads through **Resource Interpretation Definitions (RIDs)**. It acts as a layer between workload specifications and cluster scheduling, applying domain-specific optimizations.

## Core Concept: Resource Interpretation Definitions (RIDs)

RIDs are YAML manifests that teach Kai-bolt how to:
1. **Understand** workload structure (components, dependencies, scaling)
2. **Monitor** workload health (status definitions, condition mapping)
3. **Optimize** workload execution (GPU scheduling, topology, gang scheduling)

### RID Structure
```yaml
apiVersion: kai-bolt.runai.ai/v1alpha1
kind: ResourceInterpretationDefinition
metadata:
  name: framework
spec:
  kind:                    # Target Kubernetes CRD
    group: kubeflow.org
    version: v1
    kind: PyTorchJob
  
  structureDefinition:     # Component hierarchy and relationships
  - name: "pytorchjob"
    specPath: ".spec"
    kind: { group, version, kind }
    statusDefinition: { ... }
    scaleDefinition: { ... }
    dependsOn: [...]
  
  childKinds:              # Optional - custom resources needing traversal
  - group: apps
    version: v1
    kind: Deployment       # Example: Knative creates Deployments
  
  optimizationInstructions:  # Optimization directives (Iteration 02)
    gangScheduling:
      podGroups: [...]
    multiNodeNVLink:
      acceleratedComponents: [...]
```

## Key Components

### 1. Component Definition
**Purpose**: Describes the hierarchical structure of workloads, defining how Kai-bolt should interpret different parts of a Kubernetes resource.

#### Core Fields:
- **`name`**: Unique identifier for this component within the RID
- **`specPath`**: JQ expression pointing to the component's specification within the target resource
- **`referenceDefinition`**: Optional struct indicating this component represents a separate Kubernetes object
- **`kind`**: Full GroupVersionKind object identifying the Kubernetes resource type
- **`ownerName`**: Parent component name (establishes hierarchy relationships)
- **`dependsOn`**: Array of component names this component depends on

#### Component Types:
```yaml
# Root Component (Main CRD)
- name: "pytorchjob"
  specPath: ".spec"                    # Points to main spec
  kind:
    group: "kubeflow.org"
    version: "v1" 
    kind: "PyTorchJob"
  # No ownerName (root level)

# Sub-Component (Pod Template)
- name: "master"
  specPath: ".spec.pytorchReplicaSpecs.Master.template"
  ownerName: "pytorchjob"             # Child of pytorchjob
  kind:
    group: ""
    version: "v1"
    kind: "Pod"

# Reference Component (External Dependency)
- name: "nimcache-ref"
  specPath: ".spec"
  referenceDefinition:                 # Indicates separate K8s object
    componentKeyPath: ".spec.storage.nimCache.name"
  ownerName: "nimservice"
  kind:
    group: "apps.nvidia.com"
    version: "v1alpha1"
    kind: "NIMCache"
  statusDefinition: { ... }           # Required for monitoring
```

### 2. Scale Definition
**Purpose**: Defines how components can be scaled up or down for resource optimization.

#### Structure:
```yaml
scaleDefinition:
  replicasPath: ".spec.pytorchReplicaSpecs.Worker.replicas"
  minReplicasPath: ".spec.pytorchReplicaSpecs.Worker.minReplicas"   # Optional
  maxReplicasPath: ".spec.pytorchReplicaSpecs.Worker.maxReplicas"   # Optional
```

#### Use Cases:
- **Manual Scaling**: Direct replica count adjustments
- **Auto-scaling**: Min/max bounds for automatic scaling
- **Resource Planning**: Understanding capacity constraints

### 3. Status Definition
**Purpose**: Maps framework-specific Kubernetes conditions to abstract states that Kai-bolt can understand and act upon.

#### Structure:
```yaml
statusDefinition:
  conditionsDefinition:
    conditionsPath: "status.conditions"    # Where to find conditions
    typeFieldName: "type"                  # Condition type field name
    statusFieldName: "status"              # Condition status field name
  statusMappings:
    running:                               # Abstract state
      byConditions:
      - type: "Running"                    # Framework-specific condition
        status: "True"                     # Expected status value
    completed:
      byConditions:
      - type: "Succeeded"
        status: "True"
    failed:
      byConditions:
      - type: "Failed"
        status: "True"
```

#### Critical Requirements:
- **Research-Based**: Condition types must match actual framework APIs
- **Complete Coverage**: Include running, completed, and failed states
- **Framework-Specific**: Each framework has unique condition patterns

#### Framework Examples:
```yaml
# PyTorchJob Status
statusMappings:
  running:
    byConditions:
    - type: "Running"
      status: "True"
  completed:
    byConditions:
    - type: "Succeeded"
      status: "True"

# KServe Status
statusMappings:
  running:
    byConditions:
    - type: "Ready"
      status: "True"
  failed:
    byConditions:
    - type: "Ready"
      status: "False"
```

### 4. Instructions (Optimization Directives)
**Purpose**: Specify how the scheduler should optimize workload placement and execution.

**New Structure**: Instructions are now organized under `optimizationInstructions` hierarchy with no individual `enforcement` fields. Presence implies "required", absence implies "disabled".

```yaml
spec:
  optimizationInstructions:
    gangScheduling:           # Optional - only present if needed
      podGroups: [...]
    gpuInterconnect:          # Optional - only present if needed  
      acceleratedComponents: [...]
    topologyAwareness:        # Optional - only present if needed
      topologyGroups: [...]
```

#### Gang Scheduling (`gangScheduling`)
Ensures coordinated pod startup for distributed workloads:
```yaml
gangScheduling:
  podGroups:
  - name: "training-cluster"
    members:
    - componentDefinitionName: "master"
      componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
    - componentDefinitionName: "worker"
      componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
```

**Key Fields**:
- **`componentKeyPath`**: JQ expression to extract pod grouping identifier
- **Must point to resource instance names, not component types**

#### GPU Interconnect (`gpuInterconnect`)

Optimizes workloads requiring high-bandwidth GPU interconnects for multi-node GPU communication:

```yaml
gpuInterconnect:
  acceleratedComponents:
  - componentDefinitionName: "worker"
    componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
    filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
```

**Key Fields**:
- **`filter`**: JQ expression to identify GPU-enabled pods
- **Null-safe**: Uses `// 0` default for missing GPU limits
- **Component-specific**: Targets individual components, not parent resources

#### Topology Awareness (`topologyAwareness`)
Guides pod placement based on cluster topology:
```yaml
topologyAwareness:
  topologyGroups:
  - groupName: "inference-cluster"
    preferredPlacement: "node"         # node|zone
    members:
    - componentDefinitionName: "predictor"
      componentKeyPath: '.metadata.labels["serving.kserve.io/inferenceservice"]'
```

**Placement Levels**:
- **`node`**: Same physical node (latency-critical workloads)
- **`zone`**: Same availability zone (bandwidth-critical workloads)

### 5. Dependency Relationships
**Purpose**: Model explicit dependencies between components for proper orchestration.

#### Requirements:
```yaml
# Dependent component
- name: "nimservice"
  dependsOn: ["nimcache-ref"]          # Waits for cache to be ready

# Dependency component  
- name: "nimcache-ref"
  referenceDefinition:
    componentKeyPath: ".spec.storage.nimCache.name"
  statusDefinition:                    # REQUIRED for dependsOn
    statusMappings:
      running:
        byConditions:
        - type: "NIM_CACHE_JOB_COMPLETED"
          status: "True"
```

**Critical Rules**:
- Dependencies must have `statusDefinition` for monitoring
- Creates orchestration order: cache → service
- Prevents invalid startup sequences

### 6. JQ Expression Patterns
**Purpose**: Safe, reliable data extraction from Kubernetes resources.

#### Safety Requirements:
```yaml
# Null-safe GPU detection
filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'

# Proper operator precedence  
filter: '((.spec.limits["nvidia.com/gpu"] // 0) > 1)'

# Type conversion for string numbers
filter: '(.spec.limits["nvidia.com/gpu"] // "0" | tonumber) > 1'

# Component key path (instance identifier)
componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
```

#### Common Patterns:
- **Default values**: `// 0` for missing numeric fields
- **Parentheses**: Ensure correct operator precedence  
- **Type conversion**: `| tonumber` for string-to-number conversion
- **Instance identification**: Point to workload instance names, not types

### 7. Child Resources (`childKinds`)
**Purpose**: Specify custom Kubernetes resources created by the main RID kind that require operator traversal but are not modeled as explicit components.

#### Definition and Rules
`childKinds` enables the operator to read additional resources for optimization while avoiding unnecessary RBAC permissions.

**Inclusion Criteria** (ALL must be true):
1. **Created by Main Kind**: Resource has owner reference to main RID kind
2. **Custom Resource**: NOT vanilla Kubernetes (Pod, Service, ConfigMap, Secret)
3. **Compute-Related**: Actually runs containers/workloads
4. **Not Component**: NOT already in `structureDefinition`
5. **Traversal Needed**: Operator requires read access for optimization

#### Framework Examples

**Most Common: Empty childKinds**
```yaml
# When all compute resources are explicit components
# PyTorchJob, MPIJob, JobSet, LeaderWorkerSet, RayCluster, KServe, Milvus
childKinds: []  # Usually omitted for cleaner YAML
```

**Single Child Resource**
```yaml
# Knative Service: Revision creates Deployments not modeled as components
childKinds:
- group: apps
  version: v1
  kind: Deployment

# NIMCache: Creates Jobs for model operations
childKinds:
- group: batch
  version: v1
  kind: Job
```

**Multiple Child Resources**
```yaml
# NVIDIA Dynamo: Creates multiple unmodeled compute resources
childKinds:
- group: apps
  version: v1
  kind: Deployment
- group: leaderworkerset.x-k8s.io
  version: v1
  kind: LeaderWorkerSet
```

#### Research Methodology
For each framework, determine childKinds through:
1. **Study Documentation**: Review official architecture diagrams
2. **Inspect Source Code**: Analyze controller and CRD definitions
3. **Map Resource Hierarchy**: Identify all created resources
4. **Apply Filters**: Use inclusion criteria to select relevant resources
5. **Verify Gaps**: Ensure no overlap with existing components

## Component Relationships

### Hierarchy Structure
```yaml
pytorchjob (root)
├── master (child)
└── worker (child)
```

### Dependency Structure  
```yaml
nimservice → depends on → nimcache-ref
```

### Optimization Targeting
```yaml
Instructions → target → Individual Components (master, worker)
             ↙         ↘
       Simple Filters   Instance Grouping
```

This component structure enables Kai-bolt to understand complex AI/ML workloads, monitor their status accurately, and apply precise optimizations based on workload characteristics and requirements.

## Optimization Domains

### AI/ML Framework Categories
1. **Distributed Training**: PyTorch, MPI, Ray
   - Gang scheduling critical
   - Multi-GPU coordination
   - Zone-level topology preference

2. **Model Inference**: KServe, Knative, NIM
   - Low-latency optimization
   - Node-level topology preference  
   - Auto-scaling considerations

3. **Batch Processing**: JobSet, LeaderWorkerSet
   - Resource efficiency focus
   - Coordination patterns
   - Completion-based optimization

4. **Vector/Database**: Milvus, specialized storage
   - Storage locality optimization
   - Query performance focus

## Design Philosophy

1. **Architectural Accuracy**: RIDs must match actual Kubernetes resource boundaries
2. **Optimization Focus**: Components included only if they add optimization value
3. **Research-Based**: Status definitions based on actual framework APIs, not assumptions
4. **Separation of Concerns**: Complex frameworks use multiple focused RIDs
5. **Dependency Modeling**: Explicit dependencies with status monitoring

## Integration Points

- **Kubernetes API**: Designed for integration with CRD controllers
- **Cluster Schedulers**: Structured for default scheduler and specialized schedulers (Run:ai)
- **GPU Operators**: Optimizations target NVIDIA GPU resources
- **Monitoring**: Status definitions enable Kubernetes events and metrics integration

## Next Steps for New Collaborators

1. Read `rid-principles.md` for fundamental design rules
2. Review `framework-patterns.md` for AI/ML workload modeling
3. Use `validation-checklist.md` for RID quality assurance
4. Reference existing RIDs in `docs/examples/` (current implementations)
5. Follow `iteration-process.md` for development methodology