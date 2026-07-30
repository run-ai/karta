karta — workload-aware visibility for Kubernetes AI workloads
=============================================================

> **Alpha.** The CRD schema, CLI flags, and output format may change
> without notice between releases. Don't build automation against
> karta output yet.

Install
-------

Linux / macOS:

    tar -xzf karta_*.tar.gz
    sudo mv karta /usr/local/bin/karta

Windows: extract `karta.exe` from the `.zip` and place it on `PATH`.

karta uses your existing kubeconfig — no separate setup needed.

Usage
-----

    karta workload list                    # current namespace
    karta workload list -A                 # all namespaces
    karta workload list -n my-namespace    # specific namespace
    karta workload tree <name>             # hierarchical view of one workload
    karta --context my-cluster workload list

Same kubeconfig flags as kubectl: `--kubeconfig`, `--context`, `-n`,
plus `--color {auto,always,never}`.

See `karta --help` and `karta workload --help` for the full surface.

The physical layer
------------------

    karta workload tree <name> --physical

`--physical` extends the tree past the pod, to the node it landed on and
the devices it holds:

    PyTorchJob/llama-sft [Running]
    ├─ master  (1/1)  1/1 ready  gpu: 8   node-a  dev: 4   @clique-1
    │ └─ Pod/llama-sft-master-0  Running  gpu: 8   node-a  @clique-1  dev: gpu-0,gpu-1,gpu-2,gpu-3
    └─ worker  (3/3)  2/3 ready  gpu: 24  node-a,node-b,node-c  !node-c degraded  dev: 10  @clique-1,clique-2 (split)

What the annotations mean:

  * `!NotReady` / `!cordoned` — the node under that pod is degraded or
    draining. Rolled up to the component as `!<node> degraded`.
  * `@<domain>` — the node's topology domain. `(split)` on a component
    means its pods span more than one, which for a gang-scheduled
    component is a placement problem the logical tree cannot show.
  * `dev:` — the individual devices allocated to the pod. `gpu:` is what
    the pod *asked for*; `dev:` is what it actually *holds*.

Device identity requires Dynamic Resource Allocation. Without DRA a pod
records only a count (`nvidia.com/gpu: 8`), so no traversal can recover
which GPUs it got, and the node annotations render on their own.

Set `--topology-label` when your cluster names domains its own way; the
defaults are `nvidia.com/gpu.clique` then `topology.kubernetes.io/zone`.

Reads are best-effort. `--physical` needs `get` on nodes and `list` on
resourceclaims; without them the command warns and renders the logical
tree rather than failing.

Supported workload kinds
------------------------

The CLI ships with built-in Karta definitions for:

  * PyTorchJob (kubeflow.org)
  * JobSet (jobset.x-k8s.io)
  * RayCluster, RayJob (ray.io)
  * MPIJob (kubeflow.org)
  * LeaderWorkerSet (leaderworkerset.x-k8s.io)
  * InferenceService (serving.kserve.io)
  * Service (serving.knative.dev)
  * DynamoGraphDeployment (nvidia.com)
  * NIMService (apps.nvidia.com)
  * Milvus

To add a new workload kind, write a Karta definition and contribute
it under `docs/examples/` — the CLI bundle is regenerated from there.

Version & bugs
--------------

    karta version

Project: https://github.com/run-ai/karta
Issues:  https://github.com/run-ai/karta/issues

License
-------

Apache-2.0. See `LICENSE` in this archive.
