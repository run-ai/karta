# Pod Grouper Plugins - Concise Overview

## **AI/ML Framework Plugins**

### **1. Knative Plugin**
- **Groups by**: Knative Revision
- **Logic**: All pods with same `serving.knative.dev/revision` label
- **Example**: `podgroup-my-service-v1-{revision-uid}`
- **Special**: Can create individual pod groups if not gang-scheduled
- **Multiple Groups**: Backward-compatible mode creates one group per pod: `podgroup-{pod-name}-{pod-uid}`

### **2. Ray Plugin** 
- **Groups by**: RayCluster
- **Logic**: All head + worker pods in the same RayCluster
- **Example**: `podgroup-ray-cluster-{cluster-uid}`
- **MinAvailable**: Head replicas + sum of all worker group `minReplicas`
- **Multiple Groups**: Single group per RayCluster (no multi-group support)

### **3. Spark Plugin**
- **Groups by**: Spark Application
- **Logic**: All pods with same `spark-app-selector` label
- **Example**: Uses `spark-app-selector` value directly as group name
- **Special**: Groups driver + executor pods together
- **Multiple Groups**: Single group per Spark application (no multi-group support)

### **4. Kubeflow Training Jobs**

#### **PyTorch Plugin**
- **Groups by**: PyTorchJob
- **Logic**: All master + worker pods in the job
- **Example**: `podgroup-pytorch-training-{job-uid}`
- **MinAvailable**: Sum of all replica specs or `elasticPolicy.minReplicas`
- **Multiple Groups**: Single group per PyTorchJob (no multi-group support)

#### **MPI Plugin**
- **Groups by**: MPIJob
- **Logic**: All launcher + worker pods
- **Example**: `podgroup-mpi-job-{job-uid}`
- **Special**: Can exclude launcher from MinAvailable if delayed creation policy
- **Multiple Groups**: Single group per MPIJob (no multi-group support)

#### **Others** (TensorFlow, JAX, XGBoost)
- **Groups by**: Respective Job type
- **Logic**: Sum all replica specs (master, worker, etc.)
- **Example**: `podgroup-{framework}-job-{job-uid}`
- **Multiple Groups**: Single group per training job (no multi-group support)

---

## **Kubernetes Workload Plugins**

### **5. Job Plugin**
- **Groups by**: Kubernetes Job, but **per-pod groups** for parallel jobs
- **Logic**: 
  - Single job (parallelism=1): `podgroup-{job-name}-{job-uid}`
  - Parallel job: `podgroup-{pod-name}-{job-uid}`
- **Example**: `podgroup-batch-processing-abc123` or `podgroup-worker-1-abc123`
- **Multiple Groups**: Parallel jobs create multiple groups: `podgroup-worker-1-{uid}`, `podgroup-worker-2-{uid}`, etc.

### **6. Deployment Plugin**
- **Groups by**: **Individual pods** (one pod per group)
- **Logic**: Each pod gets its own pod group
- **Example**: `podgroup-{pod-name}-{pod-uid}`
- **Use case**: Web services, stateless apps
- **Multiple Groups**: Always multiple groups - one per pod: `podgroup-app-1-{uid}`, `podgroup-app-2-{uid}`, etc.

### **7. CronJob Plugin**
- **Groups by**: The underlying Job created by CronJob
- **Logic**: Finds the Job owner, then uses Job grouping logic
- **Example**: `podgroup-cronjob-run-12345-{job-uid}`
- **Multiple Groups**: Single group per job execution (follows Job plugin logic)

---

## **Specialized Platform Plugins**

### **8. Leader Worker Set Plugin**
- **Groups by**: LeaderWorkerSet
- **Logic**: All leader + worker pods in same LWS
- **Example**: `podgroup-my-training-{lws-uid}` or `podgroup-my-training-{lws-uid}-group-2`
- **Special**: Multi-group support with group index
- **Multiple Groups**: Explicit support via `replicas`: `podgroup-lws-{uid}-group-0`, `podgroup-lws-{uid}-group-1`, etc.

### **9. RunAI Job Plugin**
- **Groups by**: RunAI Job
- **Logic**: All pods belonging to the same RunAI job
- **Example**: `podgroup-{trimmed-pod-name}-{job-uid}`
- **Special**: Removes suffix from pod name
- **Multiple Groups**: Single group per RunAI job (no multi-group support)

### **10. Azure ML Plugin**
- **Groups by**: AML Job
- **Logic**: Groups by node count specified in AML job
- **Example**: `podgroup-aml-experiment-{job-uid}`
- **MinAvailable**: From `AZUREML_NODE_COUNT` environment variable
- **Multiple Groups**: Single group per AML job (no multi-group support)

### **11. Grove Plugin** (NVIDIA Internal)
- **Groups by**: Grove PodGang
- **Logic**: Uses `grove.io/podgang` label to find PodGang spec
- **Example**: `podgroup-{podgang-name}-{podgang-uid}`
- **MinAvailable**: Sum of all podReferences in PodGang
- **Multiple Groups**: Single group per PodGang (no multi-group support)

### **12. Spot Request Plugin**
- **Groups by**: SpotRequest (default grouping)
- **Logic**: Uses default grouper with inference priority
- **Example**: `podgroup-spot-job-{spotrequest-uid}`
- **Multiple Groups**: Single group per SpotRequest (no multi-group support)

---

## **Special Behavior Plugins**

### **13. PodJob Plugin**
- **Groups by**: Conditional - Spark vs Default
- **Logic**: If Spark labels exist, use Spark grouping; otherwise default
- **Example**: Spark app name OR `podgroup-{pod-name}-{pod-uid}`
- **Multiple Groups**: Depends on underlying logic (Spark = single, Default = single)

### **14. Skip Top Owner Plugin**
- **Groups by**: Second-to-last owner (skips top owner)
- **Logic**: Finds real owner by skipping wrapper resources
- **Example**: Depends on the actual owner after skipping
- **Multiple Groups**: Depends on the effective owner's plugin behavior

### **15. Default Grouper**
- **Groups by**: Top owner resource
- **Logic**: One pod group per workload (Job, Deployment, etc.)
- **Example**: `podgroup-{owner-name}-{owner-uid}`
- **MinAvailable**: 1 (single pod scheduling)
- **Multiple Groups**: Single group per top owner (no multi-group support)

---

## **Quick Reference**

| Plugin | Groups By | Group Size | Example Name | Multi-Group Support |
|--------|-----------|------------|--------------|-------------------|
| Knative | Revision | minScale | `podgroup-service-v1-{uid}` | ✅ BC mode: per-pod |
| Ray | RayCluster | head + workers | `podgroup-ray-cluster-{uid}` | ❌ Single group |
| Spark | App Selector | driver + executors | `{spark-app-selector}` | ❌ Single group |
| PyTorch | PyTorchJob | all replicas | `podgroup-pytorch-job-{uid}` | ❌ Single group |
| MPI | MPIJob | launcher + workers | `podgroup-mpi-job-{uid}` | ❌ Single group |
| Job | Job/Pod | 1 or per-pod | `podgroup-job-{uid}` | ✅ Parallel: per-pod |
| Deployment | Individual Pod | 1 | `podgroup-{pod-name}-{uid}` | ✅ Always: per-pod |
| LWS | LeaderWorkerSet | leader + workers | `podgroup-lws-{uid}-group-0` | ✅ Explicit: per-replica |
| Default | Top Owner | 1 | `podgroup-{owner-name}-{uid}` | ❌ Single group |

**Key Patterns**: 
- **ML Frameworks**: Usually single group per job for gang scheduling
- **Kubernetes Workloads**: Often multiple groups (per-pod or per-parallel-instance)
- **LWS**: Only framework with explicit multi-group design via `replicas`

## **Key Concepts**

### **Gang Scheduling**
Pod groups ensure all pods in a group are scheduled together or not at all. This prevents resource deadlocks in distributed workloads.

### **MinAvailable**
The minimum number of pods that must be schedulable before any pod in the group starts. Critical for distributed training and coordination patterns.

### **Grouping Strategies**
1. **Single Group per Workload**: Most ML frameworks (Ray, PyTorch, MPI)
2. **Multiple Groups per Workload**: LWS with replicas, parallel Kubernetes Jobs
3. **Individual Pod Groups**: Deployments, stateless services
4. **Conditional Grouping**: Knative (depends on gang scheduling config)

### **Priority Classes**
- **Training Priority**: Used for ML training workloads
- **Inference Priority**: Used for serving/inference workloads
- **Custom Priority**: Can be overridden via labels or annotations 