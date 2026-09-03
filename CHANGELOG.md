<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Changelog

Notable changes per release. This file is the in-repo source of record: a release's entry is added here before the tag is pushed (see [RELEASE.md](RELEASE.md)), and the [GitHub Release](https://github.com/run-ai/karta/releases) body is written from it, so tagged source archives carry their own entry. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) loosely. Versioning is pre-1.0: minor versions may include breaking changes, see each entry.

Scope: this file covers minor and major releases. Patch releases are cut from their release branch (`v0.1`, `v0.2`) and their entries live on that branch, so this file stays a readable history of the line rather than a log of every tag.

Entries here were backfilled from the published GitHub release notes when this file was introduced.

## v0.2.0 - 2026-07-15

Extends Karta from a CRD and Go library into an optional runnable system. No breaking API changes.

### Added

- Controller/operator: reconcile core, operator Helm chart, operator container images, and build/run Makefile targets. Karta can now run as a controller rather than only being a CRD plus library (#77, #97, #91, #96).
- `WorkloadTree`: the raw component hierarchy of a workload (desired structure, scale, specs, status) produced by the library, giving clients a single shared tree to traverse and render (#87).
- New pod-grouping API: `gangScheduling.podGroup` with subgroups and topology constraints for scheduler integrations (#145).
- `karta` CLI: entrypoint and Cobra root command (#127).
- Kind-based e2e provisioner: `hack/e2e` spins up a kind cluster for end-to-end tests (#144).
- Larger catalog of built-in workload definitions under `docs/catalog/`, spanning core, batch, training, serving, and Ray workloads (#155).
- Additional samples: Grove `PodCliqueSet` (#66), Milvus (#103), a worked controller example over LeaderWorkerSet (#92), and a minimal quickstart controller (#84).

### Changed

- Go toolchain `1.25.9` to `1.26.3`; `k8s.io/api` `v0.35.1` to `v0.36.2` (#75, #93).

### Deprecated

- The previous `gangScheduling.podGroups` grouping format. Still honored; migrate to `gangScheduling.podGroup`:

  ```diff
    optimizationInstructions:
      gangScheduling:
  -     podGroups:
  -       - name: job
  -         members:
  -           - componentName: launcher
  -             groupByKeyPaths:
  -               - .metadata.labels["training.kubeflow.org/job-name"]
  -           - componentName: worker
  -             groupByKeyPaths:
  -               - .metadata.labels["training.kubeflow.org/job-name"]
  +     podGroup:
  +       name: job
  +       subGroups:
  +         - componentName: launcher
  +         - componentName: worker
  ```

### Fixed

- Native resource discovery (#132).
- Append aliasing of caller-owned component slices (#129).
- RayJob worker `instanceIdPath` nesting in the sample (#133).
- Dependabot missing code generation failing CI (#150).

[Full changelog](https://github.com/run-ai/karta/compare/v0.1.0...v0.2.0)

## v0.1.0 - 2026-05-12

### Changed

- Breaking: project renamed from `ri`/`krt` to karta. API group is `run.ai/v1alpha1`, the CRD kind is `Karta`, and the Helm chart is named `karta` (#42, #50).
- The library no longer imports `sigs.k8s.io/controller-runtime`. Consumers that only need the CRD types or resource helpers no longer inherit it transitively (#62).

### Added

- Suspend/resume support: new `SuspendDefinition` field on the CRD and new resource statuses `Suspended`, `Suspending`, `Resuming` (#56).

[Full changelog](https://github.com/run-ai/karta/compare/v0.0.12...v0.1.0)

## v0.0.1 through v0.0.12 - 2025-09-03 to 2026-04-14

Pre-rename development releases under the earlier project name. See the [release list](https://github.com/run-ai/karta/releases) for individual notes.
