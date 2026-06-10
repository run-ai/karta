<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->
# Karta controller-runtime example

This example installs a real controller into a Kubernetes cluster (a Kind
cluster works out of the box) and reacts to live changes. It watches two
unrelated workload types, a `batch/v1` Job and a `jobset.x-k8s.io` JobSet, and
on every change uses Karta to inspect and mutate them through the exact same
reconcile code, with no CRD-specific branching.

The two CRDs are deliberately different in shape: a Job keeps its pod template
on the root object, while a JobSet keeps it inside child components and has
multiple instances. The reconciler does not know or care, because Karta resolves
where everything lives.

It complements the [quickstart](../quickstart/), which runs offline against
embedded YAML. Here the workload structure is not embedded in the binary: it
lives in the cluster as a Karta custom resource, and the controller reads it at
runtime. Adding support for another workload type needs no code change: add its
GVK to the `--watch-gvk` flag and apply its Karta object (see
[Manage another workload type](#manage-another-workload-type-no-code-change)).

## What it demonstrates

The reconcile loop in [controller.go](controller.go) does the same work for any
workload Karta can describe:

1. Discover the workload structure from the cluster by GVK: the controller lists
   Karta objects and selects the one whose root component matches the watched
   workload's GVK. It never hard-codes a Karta name, so it resolves a definition
   the same way a real consumer does for an arbitrary workload type.
2. Read a unified status (`Running`, `Completed`, `Suspended`, and so on) without
   parsing CRD-specific conditions.
3. Aggregate replica counts and container resource requests across every
   component (root and children), so the totals are right whether pods live on
   the root or in child components.
4. Write the results back as `karta/*` annotations and a Kubernetes Event.
5. Inject a pod-template label into every pod-bearing component through Karta
   `UpdatePodTemplateSpec`.

There is no `switch` on workload kind anywhere in the reconciler.

### About the pod-template mutation

A workload's pod template is immutable once it is running. It is mutable only
while the workload is suspended (`spec.suspend: true`), which holds for both Job
and JobSet. The controller handles both states honestly:

- The inspection annotations are written to the workload's own metadata, which
  is always mutable, so they appear on every workload.
- The pod-template label is injected only while the workload is suspended. For a
  running workload the controller records a `PodTemplateImmutable` event instead.

The bundled Karta definitions ([batch-job](../../samples/batch-job.yaml),
[jobset](../../samples/jobset.yaml)) already model the suspended status, so the
controller detects the mutable window with a plain `GetStatus` call.

## Prerequisites

- [kind](https://kind.sigs.k8s.io/), `kubectl`, `docker`
- Go (only to build the image)

## Run it on Kind

From the repository root.

Create a cluster:

```bash
kind create cluster
```

Install the JobSet operator (the Job type is built in; JobSet is not):

```bash
kubectl apply --server-side -f https://github.com/kubernetes-sigs/jobset/releases/download/v0.8.0/manifests.yaml
kubectl -n jobset-system rollout status deploy/jobset-controller-manager
```

Install the Karta CRD, then apply the Karta definition objects. Order matters:
the CRD must exist before the objects that use it.

```bash
kubectl apply -f charts/karta/crds/run.ai_kartas.yaml
kubectl apply -f docs/samples/batch-job.yaml
kubectl apply -f docs/samples/jobset.yaml
```

Build the controller image and side-load it into the cluster:

```bash
# Derive the Go version from go.mod so the build image stays aligned with it.
GO_VERSION=$(awk '/^go /{print $2}' docs/examples/controller-runtime/go.mod)
docker build --build-arg GO_VERSION=$GO_VERSION \
  -f docs/examples/controller-runtime/Dockerfile -t generic-controller-example:latest .
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
karta/status: Suspended
karta/replicas: 2
karta/cpu-request: 500m
karta/memory-request: 256Mi
karta/gpu-request: 0
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

Now apply the suspended JobSet. This is the point of the example: a completely
different CRD, handled by the same controller code.

```bash
kubectl apply -f docs/examples/controller-runtime/samples/jobset-suspended.yaml

# Same karta/* annotations, computed from the JobSet's own structure
kubectl get jobset karta-demo-jobset -o jsonpath='{.metadata.annotations}' | tr ',' '\n'

# The label is injected into the child replicatedJob's pod template, not the root
kubectl get jobset karta-demo-jobset \
  -o jsonpath='{.spec.replicatedJobs[0].template.spec.template.metadata.labels}'

kubectl describe jobset karta-demo-jobset | sed -n '/Events:/,$p'
```

Expected annotations (replicas = replicas x parallelism across the replicatedJob):

```text
karta/status: Suspended
karta/replicas: 4
karta/cpu-request: 250m
karta/memory-request: 128Mi
karta/gpu-request: 0
```

## Change the definition live

The controller also watches Karta objects. Editing the one whose root component
is `batch/v1 Job` re-reconciles every governed Job with the new structure, with
no redeploy:

```bash
kubectl edit karta batch-v1-job
# change something the controller reports, then save
```

This is the deeper layer of the example: the workload structure is cluster data,
not code. See [docs/samples/](../../samples/) for ready-made definitions covering
PyTorchJob, RayCluster, JobSet, and more.

## Manage another workload type (no code change)

The watched types are configured at runtime through the `--watch-gvk` flag, so
adding one never touches the Go code. To manage, say, RayCluster:

1. Make sure the CRD is installed in the cluster (the controller can only watch
   types the API server knows).
2. Apply its Karta definition, for example
   `kubectl apply -f docs/samples/raycluster.yaml`.
3. Add the GVK to the flag in `manifests/deployment.yaml` (format
   `group/version/kind`, core group empty):

   ```yaml
   args:
     - --watch-gvk=batch/v1/Job,jobset.x-k8s.io/v1alpha2/JobSet,ray.io/v1/RayCluster
   ```

4. Grant RBAC for the new resource in `manifests/rbac.yaml`:

   ```yaml
   - apiGroups: ["ray.io"]
     resources: ["rayclusters"]
     verbs: ["get", "list", "watch", "update", "patch"]
   ```

5. Re-apply the manifests: `kubectl apply -f docs/examples/controller-runtime/manifests/`.

The controller creates one watch/reconciler per configured GVK at startup, so a
flag change takes effect on the next rollout. The reconcile code is untouched;
Karta absorbs the new workload's structure.

## File layout

| File | Purpose |
|------|---------|
| `main.go` | Manager setup: scheme, one reconciler per watched GVK, start |
| `controller.go` | `WorkloadReconciler` inspect + mutate loop (no per-CRD branching) |
| `Dockerfile` | Builds the controller image from the repository root |
| `manifests/rbac.yaml` | ServiceAccount, ClusterRole, ClusterRoleBinding |
| `manifests/deployment.yaml` | Controller Deployment in the `karta-system` namespace |
| `samples/job-suspended.yaml` | Job created suspended (mutation path) |
| `samples/job-running.yaml` | Running Job (inspect-only, immutability boundary) |
| `samples/jobset-suspended.yaml` | JobSet created suspended (multi-component, child pod template) |

## Clean up

```bash
kubectl delete -f docs/examples/controller-runtime/samples/ --ignore-not-found
kubectl delete -f docs/examples/controller-runtime/manifests/ --ignore-not-found
kind delete cluster
```
