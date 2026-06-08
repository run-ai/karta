<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->
# Karta controller-runtime example

This example installs a real controller into a Kubernetes cluster (a Kind
cluster works out of the box) and reacts to live changes. On every change to a
`batch/v1` Job, the controller uses Karta to inspect the workload and to mutate
it, with no CRD-specific code in the reconcile loop.

It complements the [quickstart](../quickstart/), which runs offline against
embedded YAML. Here the workload structure is not embedded in the binary: it
lives in the cluster as a Karta custom resource, and the controller reads it at
runtime. Adding support for another workload type means applying a new Karta
object, not recompiling the controller.

## What it demonstrates

The reconcile loop in [controller.go](controller.go) does the same work for any
workload Karta can describe:

1. Discover the workload structure from the cluster by GVK: the controller lists
   Karta objects and selects the one whose root component matches the watched
   workload's GVK. It never hard-codes a Karta name, so it resolves a definition
   the same way a real consumer does for an arbitrary workload type.
2. Read a unified status (`Running`, `Completed`, `Suspended`, and so on) without
   parsing CRD-specific conditions.
3. Read the replica count and aggregate container resource requests.
4. Write the results back as `karta.run.ai/*` annotations and a Kubernetes Event.
5. Inject a pod-template label through Karta `UpdatePodTemplateSpec`.

There is no `switch` on workload kind anywhere in the reconciler.

### About the pod-template mutation

A `batch/v1` Job's `spec.template` is immutable once the Job is running. It is
mutable only while the Job is suspended (`spec.suspend: true`). The controller
handles both cases honestly:

- The inspection annotations are written to the Job's own metadata, which is
  always mutable, so they appear on every Job.
- The pod-template label is injected only while the Job is suspended. For a
  running Job the controller records a `PodTemplateImmutable` event instead.

The bundled [batch-job Karta definition](../../samples/batch-job.yaml) already
models the suspended status, so the controller detects the mutable window with a
plain `GetStatus` call.

## Prerequisites

- [kind](https://kind.sigs.k8s.io/), `kubectl`, `docker`
- Go (only to build the image)

## Run it on Kind

From the repository root.

Create a cluster:

```bash
kind create cluster
```

Install the Karta CRD, then apply the Karta definition object. Order matters:
the CRD must exist before the object that uses it.

```bash
kubectl apply -f charts/karta/crds/run.ai_kartas.yaml
kubectl apply -f docs/samples/batch-job.yaml
```

Build the controller image and side-load it into the cluster:

```bash
docker build -f docs/examples/controller-runtime/Dockerfile -t generic-controller-example:latest .
kind load docker-image generic-controller-example:latest
```

Deploy the controller:

```bash
kubectl apply -f docs/examples/controller-runtime/manifests/
kubectl -n karta-system rollout status deploy/generic-controller
```

## See it work

Apply the suspended Job and inspect what the controller wrote:

```bash
kubectl apply -f docs/examples/controller-runtime/samples/job-suspended.yaml

# Karta-derived annotations on the Job metadata
kubectl get job karta-demo-suspended -o jsonpath='{.metadata.annotations}' | tr ',' '\n'

# The injected pod-template label (Job is suspended, so the template is mutable)
kubectl get job karta-demo-suspended -o jsonpath='{.spec.template.metadata.labels}'

# Events recorded by the controller
kubectl describe job karta-demo-suspended | sed -n '/Events:/,$p'
```

Expected annotations:

```text
karta.run.ai/status: Suspended
karta.run.ai/replicas: 2
karta.run.ai/cpu-request: 500m
karta.run.ai/memory-request: 256Mi
karta.run.ai/gpu-request: 0
```

Resume the Job and watch the injected label reach the pods:

```bash
kubectl patch job karta-demo-suspended -p '{"spec":{"suspend":false}}'
kubectl get pods -l app.kubernetes.io/managed-by=karta
```

Apply the running Job to see the immutability boundary:

```bash
kubectl apply -f docs/examples/controller-runtime/samples/job-running.yaml
kubectl describe job karta-demo-running | sed -n '/Events:/,$p'
```

The annotations are still written, and a `PodTemplateImmutable` event explains
why no pod-template label was injected.

## Change the definition live

The controller also watches Karta objects. Editing the one whose root component
is `batch/v1 Job` re-reconciles every governed Job with the new structure, with
no redeploy:

```bash
kubectl edit karta batch-v1-job
# change something the controller reports, then save
```

This is the deeper layer of the example: the workload structure is cluster data,
not code. Supporting a new workload type is an `apply`, not a rebuild. See
[docs/samples/](../../samples/) for ready-made definitions covering PyTorchJob,
RayCluster, JobSet, and more.

## File layout

| File | Purpose |
|------|---------|
| `main.go` | Manager setup: scheme, watches, start |
| `controller.go` | `JobReconciler` inspect + mutate loop (no per-CRD branching) |
| `Dockerfile` | Builds the controller image from the repository root |
| `manifests/rbac.yaml` | ServiceAccount, ClusterRole, ClusterRoleBinding |
| `manifests/deployment.yaml` | Controller Deployment in the `karta-system` namespace |
| `samples/job-suspended.yaml` | Job created suspended (mutation path) |
| `samples/job-running.yaml` | Running Job (inspect-only, immutability boundary) |

## Clean up

```bash
kubectl delete -f docs/examples/controller-runtime/samples/ --ignore-not-found
kubectl delete -f docs/examples/controller-runtime/manifests/ --ignore-not-found
kind delete cluster
```
