# RID Design Principles

## Non-Negotiable Architectural Rules

### 1. **Dependencies Require Status Definitions**
Any component referenced in `dependsOn` MUST have a `statusDefinition`.

**✅ Correct**: 
```yaml
- name: "nimservice"
  dependsOn: ["nimcache-ref"]

- name: "nimcache-ref" 
  referenceDefinition:
    componentKeyPath: ".spec.storage.cache.name"
  statusDefinition:    # ← REQUIRED for dependsOn to work
    statusMappings:
      running:
        byConditions:
        - type: "CACHE_READY"
```

**❌ Wrong**: `dependsOn` without status monitoring has no way to check readiness

### 2. **kind Fields Must Be Full GVK Objects**
Never use strings for `kind` fields - always use complete GroupVersionKind structure.

**✅ Correct**:
```yaml
kind:
  group: "kubeflow.org"
  version: "v1"
  kind: "PyTorchJob"
```

**❌ Wrong**:
```yaml
kind: "PyTorchJob"  # String - missing group/version
```

### 3. **componentKeyPath Points to Owning Resource Names**
Must point to the actual resource instance name, not component types.

**✅ Correct**:
```yaml
componentKeyPath: '.metadata.labels["training.kubeflow.org/job-name"]'
# Returns: "my-pytorch-job-123"
```

**❌ Wrong**:
```yaml
componentKeyPath: '.metadata.labels["training.kubeflow.org/replica-type"]'  
# Returns: "Master" - this is a type, not an instance identifier
```

### 4. **Research-Based Status Definitions**
Status conditions must match actual framework APIs, not assumptions.

**✅ Correct**: Based on actual PyTorchJob API
```yaml
statusMappings:
  running:
    byConditions:
    - type: "Running"      # Actual PyTorchJob condition
      status: "True"
  completed:
    byConditions:
    - type: "Succeeded"    # Actual PyTorchJob condition
      status: "True"
```

**❌ Wrong**: Generic assumptions
```yaml
statusMappings:
  running:
    byConditions:
    - type: "Active"       # Generic - doesn't exist in PyTorchJob
      status: "True"
```

### 5. **Child Specification Patterns**
Use appropriate pattern based on how framework embeds pod specifications.

**PodTemplateSpec Pattern** (preferred):
```yaml
childSpecDefinition:
  podTemplateSpecPath: ".spec.pytorchReplicaSpecs.Master.template"  # Absolute path
```

**Fragmented Pattern** (when properties are scattered):
```yaml
childSpecDefinition:
  fragmentedPodDefinition:
    labelsPath: ".spec.labels"                    # Absolute path
    resourcesPath: ".spec.resources"              # Absolute path
    schedulerNamePath: ".spec.schedulerName"     # Absolute path
```

### 6. **Absolute Path Requirement**
All paths within `childSpecDefinition` must be absolute from root RID resource.

**✅ Correct**:
```yaml
childSpecDefinition:
  podTemplateSpecPath: ".spec.pytorchReplicaSpecs.Master.template"  # From PyTorchJob root
```

**❌ Wrong**:
```yaml
childSpecDefinition:
  podTemplateSpecPath: ".template"  # Relative path - ambiguous
```

### 7. **Controlling vs Generated Components**
`childSpecDefinition` belongs on controlling components, not generated ones.

**✅ Controlling Component** (optimization target):
```yaml
- name: "service"
  childSpecDefinition:
    podTemplateSpecPath: ".spec.template"
```

**✅ Generated Component** (no optimization):
```yaml
# In referencedComponents section - indicates external resource
- name: "revision"
  # No childSpecDefinition - generated/read-only
  # No ownerName - external to main workload
```

**Note**: Components are now categorized by their placement in `rootComponent`, `childComponents`, or `referencedComponents` sections rather than using flags.

### 8. **additionalChildKinds Exclusion Rule**
Must exclude resource types that have explicit component definitions.

**✅ Correct**:
```yaml
additionalChildKinds:
- group: apps
  version: v1
  kind: Deployment  # No explicit Deployment component

components:
- name: "worker"
  kind:
    group: apps
    version: v1
    kind: StatefulSet  # Has explicit component - NOT in additionalChildKinds
```

**❌ Wrong**:
```yaml
additionalChildKinds:
- group: apps
  version: v1
  kind: StatefulSet    # Also has explicit component - duplicated!
```

## Field Structure Requirements

### **Component Definition Fields**
- **`name`** - Required string identifier for the component
- **`kind`** - Required GVK specification (group, version, kind)
- **`ownerName`** - Required for child components, nil for root
- **`metadataPath`** - Optional JQ path to metadata (defaults to `.metadata`)
- **`specDefinition`** - Required for child components (renamed from `childSpecDefinition`)
- **`scaleDefinition`** - Optional scaling configuration
- **`statusDefinition`** - Required for root components only
- **`podSelector`** - Optional pod identification for multi-component RIDs
- **`references`** - Optional list of referenced components

### **SpecDefinition Requirements**
- **One of three patterns must be specified**:
  - `podTemplateSpecPath` - JQ path to complete pod template spec
  - `podSpecPath` - JQ path to pod spec
  - `fragmentedPodDefinition` - Scattered pod specification fields
- **`specPath` field is deprecated** - No longer needed after adding specDefinition

### **FragmentedPodDefinition Fields**
- **Core fields**: `labelsPath`, `annotationsPath`, `resourcesPath`, `containersPath`
- **Scheduling fields**: `schedulerNamePath`, `podAffinityPath`, `nodeAffinityPath`
- **Enhanced fields**: `priorityClassNamePath`, `imagePath`, `resourceClaimsPath`
- **All fields are optional** - Using `omitempty` for flexibility

### **PodSelector Configuration**
Used to identify which component type a specific pod belongs to in multi-component RIDs:

```yaml
podSelector:
  keyPath: '.metadata.labels["training.kubeflow.org/replica-type"]'
  value: "master"  # Optional - if nil, checks for key existence only
```

**Required for**: RIDs with multiple components that have `specDefinition` (PyTorchJob, MPIJob, RayCluster, KServe, Milvus, LWS)

**Not needed for**: Single-component RIDs (JobSet, NIMService, NIMCache, Dynamo, Knative)

**Mutually exclusive requirement**: Pod selectors within the same RID must be mutually exclusive to avoid ambiguity.

## Component Design Rules

### 1. **Owner Hierarchies Are Explicit**
Use `ownerName` to establish clear parent-child relationships.

```yaml
- name: "pytorchjob"    # Parent
  # ... no ownerName (root)

- name: "master"        # Child  
  ownerName: "pytorchjob"
  
- name: "worker"        # Child
  ownerName: "pytorchjob"
```

### 2. **Root Components Use `.spec` specPath**
Main CRD components should consistently use `.spec` as their specPath.

**✅ Standard Pattern**:
```yaml
- name: "pytorchjob"
  specPath: ".spec"      # Consistent across all frameworks
```

### 3. **Component Inclusion Principle**
Include components only if they provide additional optimization granularity.

**Include**:
- ✅ Main CRDs (job-level operations)
- ✅ Scalable compute components (pods with different resource profiles)
- ✅ Dependencies (referenced resources)

**Exclude**:
- ❌ Services (networking, not compute optimization)
- ❌ ConfigMaps (configuration, not workload optimization)
- ❌ 1:1 mappings that don't add optimization value

## Instruction Design Rules

### 1. **Target Individual Components with Simple Filters**
Avoid complex OR logic on parent components.

**✅ Correct**: Target specific components
```yaml
optimizationInstructions:
  multiNodeNVLink:
    acceleratedComponents:
    - componentDefinitionName: "master"
      filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
    - componentDefinitionName: "worker"  
      filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
```

**❌ Wrong**: Complex filters on parent
```yaml
optimizationInstructions:
  multiNodeNVLink:
    acceleratedComponents:
    - componentDefinitionName: "pytorchjob"
      filter: 'any(.spec.pytorchReplicaSpecs[]; (.template.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0)'
```

### 2. **One Instruction Per Type**
Use single instruction instances with multiple members, not multiple instructions.

**✅ Correct**:
```yaml
optimizationInstructions:
  gangScheduling:  # Single instruction type
    podGroups:
    - name: "training-cluster"
      members:
      - componentDefinitionName: "master"
      - componentDefinitionName: "worker"
```

**❌ Wrong**: Multiple instruction instances
```yaml
instructions:
- name: "GangSchedulingInstruction"  # First instance
  podGroups: [master]
- name: "GangSchedulingInstruction"  # Second instance - wrong!
  podGroups: [worker]
```

### 3. **Framework-Appropriate Topology**
Choose topology levels based on workload characteristics.

- **Distributed Training**: `zone` (cross-node bandwidth critical)
- **Model Inference**: `node` (latency critical)  
- **Batch Processing**: `zone` (resource efficiency)
- **Caching/Storage**: `zone` (storage locality)

## JQ Expression Safety

### 1. **Null Safety with Default Values**
Always provide defaults for optional fields.

```yaml
filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
#                                                                ^^^^
#                                                           Default to 0
```

### 2. **Parentheses for Operator Precedence**
Wrap default operations to ensure correct parsing.

**✅ Correct**:
```yaml
filter: '((.spec.limits["nvidia.com/gpu"] // 0) > 0)'
#         ^                               ^
#         Ensures (default value) > 0, not default (value > 0)
```

### 3. **Type Conversion for String Numbers**
Use `tonumber` when comparing string values.

```yaml
filter: '(.spec.limits["nvidia.com/gpu"] // "0" | tonumber) > 1'
```

## Validation Requirements

Every RID must pass these checks:

1. **All `kind` fields are full GVK objects**
2. **All `componentKeyPath` point to resource names**
3. **All `dependsOn` components have `statusDefinition`**
4. **All `ownerName` values reference existing component names**
5. **All status conditions match actual framework APIs**
6. **All JQ expressions are null-safe with proper defaults**
7. **All `specDefinition` paths are absolute from root resource**
8. **`specDefinition` only on controlling components, not generated**
9. **`additionalChildKinds` excludes resource types with explicit components**
10. **Serving resources have `specDefinition`, management resources do not**
11. **Multi-component RIDs have mutually exclusive `podSelector` definitions**
12. **Pod selectors reference actual framework-generated labels/annotations**

## Complex Framework Patterns

### Multi-CRD Frameworks (like NIM)
When frameworks use multiple independent CRDs with different optimization domains:

1. **Create separate RIDs** for each CRD
2. **Model dependencies** using `referenceDefinition` struct and `dependsOn`
3. **Focus optimizations** on each CRD's specific domain

**Example**: NIM framework
- `nimcache.yaml`: Batch download optimization
- `nimservice.yaml`: GPU inference optimization with cache dependency

This separation provides:
- ✅ Domain-specific optimization
- ✅ Independent lifecycle management  
- ✅ Clear architectural boundaries
- ✅ Proper dependency orchestration 