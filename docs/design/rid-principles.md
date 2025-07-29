# RID Design Principles

## 🚨 Non-Negotiable Architectural Rules

### 1. **Match Kubernetes Resource Boundaries**
RIDs must reflect actual Kubernetes CRD structures, not idealized abstractions.

**✅ Correct**: Separate RIDs for separate CRDs
```yaml
# nimcache.yaml - for NIMCache CRD
kind:
  group: apps.nvidia.com
  version: v1alpha1  
  kind: NIMCache

# nimservice.yaml - for NIMService CRD  
kind:
  group: apps.nvidia.com
  version: v1alpha1
  kind: NIMService
```

**❌ Wrong**: Single RID trying to model multiple unrelated CRDs

### 2. **Dependencies Require Status Definitions**
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

### 3. **kind Fields Must Be Full GVK Objects**
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

### 4. **componentKeyPath Points to Owning Resource Names**
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

### 5. **Research-Based Status Definitions**
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

## 🎯 Component Design Rules

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

## 🔧 Instruction Design Rules

### 1. **Target Individual Components with Simple Filters**
Avoid complex OR logic on parent components.

**✅ Correct**: Target specific components
```yaml
optimizationsInstructions:
  multiNodeNVLink:
    acceleratedComponents:
    - componentDefinitionName: "master"
      filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
    - componentDefinitionName: "worker"  
      filter: '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'
```

**❌ Wrong**: Complex filters on parent
```yaml
optimizationsInstructions:
  multiNodeNVLink:
    acceleratedComponents:
    - componentDefinitionName: "pytorchjob"
      filter: 'any(.spec.pytorchReplicaSpecs[]; (.template.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0)'
```

### 2. **One Instruction Per Type**
Use single instruction instances with multiple members, not multiple instructions.

**✅ Correct**:
```yaml
optimizationsInstructions:
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

## 🛡️ JQ Expression Safety

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

## 📋 Validation Requirements

Every RID must pass these checks:

1. **All `kind` fields are full GVK objects**
2. **All `componentKeyPath` point to resource names**
3. **All `dependsOn` components have `statusDefinition`**
4. **All `ownerName` values reference existing component names**
5. **All status conditions match actual framework APIs**
6. **All JQ expressions are null-safe with proper defaults**

## 🎪 Complex Framework Patterns

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