<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta

**A standard way to describe the structure of any Kubernetes workload type.**

Karta lets you define a portable, declarative blueprint for any Kubernetes workload — whether it's a simple Deployment, a distributed PyTorchJob, or a custom CRD. Controllers and platforms can then use that blueprint to inspect, modify, and manage workloads without hard-coding knowledge of each type.

## The Problem

Every Kubernetes workload type (Job, Deployment, RayCluster, PyTorchJob, KServe InferenceService, ...) has a different structure. If you're building a controller, scheduler, or platform that needs to work with multiple workload types, you end up writing bespoke logic for each one:

- Where is the pod template?
- How do I find the replica count?
- Which status conditions mean "running" vs "failed"?
- How do I modify the pod spec without breaking the workload?

This doesn't scale. Every new workload type means new code, and the ecosystem is feeling it:

- [Kueue](https://github.com/kubernetes-sigs/kueue) maintains 10-12 separate per-CRD integrations (~200-500 lines each), each implementing the same `GenericJob` interface. IBM built [AppWrapper](https://github.com/project-codeflare/appwrapper) specifically to escape this burden.
- [Kubeflow](https://github.com/kubeflow/trainer) originally shipped separate operators per ML framework (tf-operator, pytorch-operator, mpi-operator, ...) and spent years migrating to a unified TrainJob v2 — but that only covers training, not inference, serving, or custom CRDs.
- [Volcano](https://github.com/volcano-sh/volcano) and [KAI Scheduler](https://github.com/NVIDIA/KAI-Scheduler) each maintain their own per-framework scheduling integrations.

The Kubernetes [Workload API (KEP-4671)](https://github.com/kubernetes/enhancements/issues/4671) addresses gang scheduling but requires every workload controller to explicitly create `Workload` objects — a high adoption barrier that doesn't help existing workloads already running in clusters.

## The Solution

Karta introduces the **Resource Interface (RI)** — a CRD that maps the structure of any workload type into a standard schema. Define it once, and any controller can use it to:

- **Extract** pod templates, replica counts, status, and metadata
- **Update** pod specs, labels, and annotations across all instances
- **Understand** workload hierarchy (e.g., a JobSet with master + worker groups)

```
┌─────────────────────────────────────────────────┐
│                   Your Platform                  │
│  (scheduler, controller, dashboard, CLI, etc.)   │
├─────────────────────────────────────────────────┤
│              Karta Component API                 │
│    Extract pods · Update specs · Read status     │
├──────────┬──────────┬──────────┬────────────────┤
│ RI:      │ RI:      │ RI:      │ RI:            │
│ JobSet   │ RayCluster│PyTorchJob│ YourCustomCRD │
└──────────┴──────────┴──────────┴────────────────┘
```

## Quick Start

### Install the CRD

```bash
kubectl apply -f https://raw.githubusercontent.com/run-ai/karta/main/charts/ri/crds/optimization.nvidia.com_resourceinterfaces.yaml
```

### Use the Go library

```bash
go get github.com/run-ai/karta@latest
```

### Define a Resource Interface

Here's an RI for a JobSet — a distributed training workload with master and worker groups:

```yaml
apiVersion: optimization.nvidia.com/v1alpha1
kind: ResourceInterface
spec:
  structureDefinition:
    rootComponent:
      name: jobset
      kind:
        group: jobset.x-k8s.io
        version: v1alpha2
        kind: JobSet
      statusDefinition:
        conditionsDefinition:
          path: .status.conditions
          typeFieldName: type
          statusFieldName: status
        statusMappings:
          running:
          - byConditions:
            - type: StartupPolicyCompleted
              status: "True"
          completed:
          - byConditions:
            - type: Completed
              status: "True"
          failed:
          - byConditions:
            - type: Failed
              status: "True"

    childComponents:
    - name: replicatedjob
      kind:
        group: batch
        version: v1
        kind: Job
      ownerRef: jobset
      specDefinition:
        podTemplateSpecPath: .spec.replicatedJobs[].template.spec.template
      scaleDefinition:
        replicasPath: .spec.replicatedJobs[].replicas
      instanceIdPath: .spec.replicatedJobs[].name  # Instances: "master", "worker"
```

### Extract workload information

```go
import "github.com/run-ai/karta/pkg/resource"

// Create a factory from your ResourceInterface and workload object
factory := resource.NewComponentFactoryFromObject(resourceInterface, jobSetObject)

// Get the child component which has the per-instance data
component, _ := factory.GetComponent("replicatedjob")
summaries, _ := component.GetExtractedInstances(ctx)

// Access pod template specs, metadata, and scale info for each instance
for instanceID, summary := range summaries {
    // instanceID will be "master" or "worker"
    if summary.PodTemplateSpec != nil {
        fmt.Printf("Instance %s image: %s\n", instanceID, summary.PodTemplateSpec.Spec.Containers[0].Image)
    }
}

// Get status from the root component
rootComponent, _ := factory.GetRootComponent()
status, _ := rootComponent.GetStatus(ctx)
// status.MatchedStatuses: matched statuses based on conditions (e.g., ["running"])
// status.Phase: raw phase string from the workload
// status.Conditions: []Condition with Type, Status, Message fields
```

### Update workload specs

The same paths defined in `specDefinition` are used for both extraction and updates:

```go
// Prepare updates per instance
updates := map[string]resource.FragmentedPodSpec{
    "master": {
        SchedulerName: "my-custom-scheduler",
        Labels: map[string]string{"my-label": "true"},
    },
    "worker": {
        SchedulerName: "my-custom-scheduler",
    },
}

// Apply updates — modifies the underlying unstructured object
err := component.UpdateFragmentedPodSpec(ctx, updates)

// Get the updated object to apply back to the cluster
updatedObject, _ := factory.GetObject()
```

## Supported Workload Types

Karta ships with RI definitions for 11+ workload types, with more being added:

| Workload Type | Framework |
|---|---|
| JobSet | Kubernetes |
| PyTorchJob | Kubeflow |
| RayCluster | Ray |
| RayJob | Ray |
| InferenceService | KServe |
| Knative Service | Knative |
| MPIJob | Kubeflow |
| NIM Service | NVIDIA |
| LeaderWorkerSet | Kubernetes |
| Milvus | Milvus |
| DynamoGraphDeployment | NVIDIA Dynamo |

See [`docs/examples/`](docs/examples/) for the full RI definitions.

### Complex example: NVIDIA Dynamo

The [Dynamo RI](docs/examples/dynamo.yaml) shows Karta handling a real-world multi-service inference graph — fragmented pod specs across services, autoscaling with min/max replicas, replica selectors for multi-node workers, gang scheduling, and 6 additional child resource types (DynamoComponentDeployment, LeaderWorkerSet, PodGang, PodClique, PodCliqueSet, PodCliqueScalingGroup). A single RI definition replaces what would otherwise require hundreds of lines of per-type controller logic.

## Who Uses Karta?

Karta was created at [Run:ai](https://run.ai) (NVIDIA) to power workload management across diverse Kubernetes workload types. It is used internally by multiple services including the workload controller, scheduler integrations, and platform components.

## Documentation

- [Technical Guide](docs/Technical%20Guide.md) — Full RI spec, path syntax (jq), validation rules
- [Examples](docs/examples/) — Real-world RI definitions for common workload types
- [API Reference](https://pkg.go.dev/github.com/run-ai/karta) — Go package documentation
- [CONTRIBUTING.md](CONTRIBUTING.md) — How to contribute (DCO required)

## Status

Karta is in active development (pre-1.0). The API may change between minor versions. We welcome feedback and contributions — please open an issue or start a discussion.

## License

Apache License 2.0 — see [LICENSE](LICENSE) for the full text.

Copyright (c) 2026 NVIDIA Corporation. See [NOTICE](NOTICE) for third-party attributions.
