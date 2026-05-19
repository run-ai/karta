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
