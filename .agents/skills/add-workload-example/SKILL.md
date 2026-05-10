---
name: add-workload-example
description: >-
  Add a new pre-built Karta example for a workload type (e.g. a custom
  CRD or upstream workload) under docs/examples/. Walks the contributor
  through reading the workload's CRD schema, mapping its components and
  fields to the Karta structureDefinition, validating the YAML, and
  opening a PR. Use when adding a new workload type to docs/examples/.
user-invocable: true
---

# Add a Karta Workload Example

Use this skill when a contributor wants to publish a Karta definition for a workload type that does not yet have one under `docs/examples/`. The output is a new YAML file that maps the workload's structure (root + child components, replica paths, status mappings, pod selectors) into the Karta CRD.

## Prerequisites

- A reachable copy of the workload's CRD (or a sample manifest of an instance) so structure can be confirmed against the real schema.
- An open GitHub issue for the new workload type, or willingness to file one before opening a PR.
- Read `docs/Technical Guide.md` once if unfamiliar with Karta's structure model (root component, child components, JQ paths, status definitions).

## Steps

### 1. Confirm the workload is not already covered

```bash
ls docs/examples/
```

If a file exists for this workload type or a close variant, prefer extending the existing example over adding a new one. Karta examples are keyed by `metadata.name` of the form `<group>-<kind-lc>-<version>`; check there too.

### 2. Pick a reference example

Choose the closest existing example as a starting template. Common starting points:

| If your workload is... | Use as template |
|------------------------|-----------------|
| A single pod with replicas (Deployment-like) | `knative-serving.yaml` |
| A driver + workers split (PyTorchJob, MPIJob) | `pytorch.yaml` or `mpijob.yaml` |
| A multi-replica-spec layout (head + worker groups) | `raycluster.yaml` |
| A serving stack with multiple sub-resources | `kserve.yaml` or `nimservice.yaml` |
| Sub-jobs / nested CRDs (job + pods) | `jobset.yaml` or `rayjob.yaml` |
| Array of named tasks/replicas (Volcano `spec.tasks`, JobSet `replicatedJobs`) | `pytorch.yaml` shape with `select(.name == "...")` JQ filters per child component |

Copy the chosen template to `docs/examples/<workload-name>.yaml`.

### 3. Fill in the root component

```yaml
spec:
  structureDefinition:
    rootComponent:
      name: <workload-name>
      kind:
        group: <api-group>
        version: <api-version>
        kind: <Kind>
      statusDefinition:
        conditionsDefinition:
          path: .status.conditions
          typeFieldName: type
          statusFieldName: status
          messageFieldName: message
        statusMappings:
          initializing: [...]
          running: [...]
          completed: [...]
          failed: [...]
```

Required fields:
- `name`: short identifier, lowercase. Used in the resource graph.
- `kind`: GVK of the workload's top-level CR.
- `statusDefinition.conditionsDefinition.path`: JQ path to the conditions array. Adjust if the CRD uses a non-standard layout.
- `statusMappings`: map each Karta phase (`initializing`, `running`, `completed`, `failed`) to one or more `byConditions` rules.

If the workload exposes a phase string (e.g. `.status.state.phase`) instead of `status.conditions[]`, replace `conditionsDefinition` with `phaseDefinition` and map values via `byPhase`. See `dynamo.yaml` and `raycluster.yaml`. For more complex status logic that neither pattern handles, fall back to `statusMappings.byJQ` with a JQ expression that returns the phase string.

### 4. Define child components

For each pod-producing piece of the workload (master, worker, head, ray-worker-group, etc.):

```yaml
- name: <component-name>
  kind:
    group: ""
    version: v1
    kind: Pod
  ownerRef: <root-component-name>
  specDefinition:
    podTemplateSpecPath: <jq path to the PodTemplateSpec>
  scaleDefinition:
    replicasPath: <jq path to the replica count, with default e.g. // 1>
  podSelector:
    componentTypeSelector:
      keyPath: <jq path on pod metadata that distinguishes this component>
      value: <expected value>
```

Common selectors:
- Kubeflow training-operator workloads: `.metadata.labels["training.kubeflow.org/replica-type"]`
- Ray clusters: `.metadata.labels["ray.io/group"]`
- JobSet: `.metadata.labels["jobset.sigs.k8s.io/replicatedjob-name"]`
- Custom CRDs: usually a label set by the workload's controller; inspect a live pod to confirm.

### 5. Validate the JQ paths

Before committing, verify each path against a live sample of the workload:

```bash
# Apply a sample workload manifest to a kind cluster
kubectl apply -f my-sample.yaml

# Confirm the conditions path
kubectl get <kind> <name> -o json | jq '.status.conditions'

# Confirm the pod template path
kubectl get <kind> <name> -o json | jq '.spec.<...>.template'

# Confirm the replicas path
kubectl get <kind> <name> -o json | jq '.spec.<...>.replicas // 1'
```

If a path returns `null` or the wrong shape, fix it before opening the PR.

### 6. Add a YAML header

Every new file under `docs/examples/` includes the SPDX + copyright header:

```yaml
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
```

### 7. Run validation

```bash
make validate    # confirms generated artifacts are clean
make lint        # confirms YAML/Go style across the repo
```

If `make validate` complains, the example file may have triggered regeneration of a related artifact. Commit those changes alongside the example.

### 8. Open the PR

- Title: `feat(examples): add Karta example for <workload-name>`
- Reference the GitHub issue in the body.
- In the PR description, include:
  - The CRD GVK
  - The components defined (root + each child)
  - A short note on how the example was validated (kind cluster, sample manifest, etc.)
  - Any limitations (e.g. status mappings only cover the common phases)

Mark the PR as draft if validation against a real cluster has not yet been done.

## Reference

- Existing examples: `docs/examples/`
- Karta CRD schema: `pkg/api/runai/v1alpha1/`
- JQ path engine: `pkg/jq/`
- Technical Guide: `docs/Technical Guide.md`
