<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Changelog

Notable changes per release. This file is the in-repo source of record: a release's entry is added here before the tag is pushed (see [RELEASE.md](RELEASE.md)), and the [GitHub Release](https://github.com/run-ai/karta/releases) body is written from it, so tagged source archives carry their own entry. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) loosely. Versioning is pre-1.0: minor versions may include breaking changes, see each entry.

Entries through v0.2.3 were backfilled from the published GitHub release notes when this file was introduced.

## v0.2.3 - 2026-08-17

### Added

- Digest-pinned air-gap image locks per release: per-platform `ImageLock` YAML assets (linux/amd64, linux/arm64) listing the exact image digests needed for air-gapped installs (#237).

[Full changelog](https://github.com/run-ai/karta/compare/v0.2.2...v0.2.3)

## v0.2.2 - 2026-07-23

### Fixed

- CRD cache transform fix for the v0.2 controller (#177).

[Full changelog](https://github.com/run-ai/karta/compare/v0.2.1...v0.2.2)

## v0.2.1 - 2026-07-20

### Fixed

- Helm chart: operator memory limit raised to 256Mi (#164).

[Full changelog](https://github.com/run-ai/karta/compare/v0.2.0...v0.2.1)

## v0.2.0 - 2026-07-15

Extends Karta from a CRD and Go library into an optional runnable system. No breaking API changes.

### Added

- Controller/operator: reconcile core, operator Helm chart, and container images. Karta can now run as a controller rather than only being a CRD plus library (#77, #97, #91).
- `WorkloadTree`: the raw component hierarchy of a workload (desired structure, scale, specs, status) produced by the library, giving clients a single shared tree to traverse and render (#87).
- New pod-grouping API: `gangScheduling.podGroup` with subgroups and topology constraints for scheduler integrations (#145).
- Larger catalog of built-in workload definitions under `docs/catalog/` (#155).

### Deprecated

- The previous `gangScheduling.podGroups` grouping format. Still honored; migrate to `gangScheduling.podGroup`.

[Full changelog](https://github.com/run-ai/karta/compare/v0.1.1...v0.2.0)

## v0.1.1 - 2026-06-01

### Changed

- Go toolchain bumped to 1.26.3; dependencies updated (#82).

[Full changelog](https://github.com/run-ai/karta/compare/v0.1.0...v0.1.1)

## v0.1.0 - 2026-05-12

### Changed

- Breaking: project renamed from `ri`/`krt` to karta. API group is `run.ai/v1alpha1`, the CRD kind is `Karta`, and the Helm chart is named `karta` (#42, #50).
- The library no longer imports `sigs.k8s.io/controller-runtime`. Consumers that only need the CRD types or resource helpers no longer inherit it transitively (#62).

### Added

- Suspend/resume support: new `SuspendDefinition` field on the CRD and new resource statuses `Suspended`, `Suspending`, `Resuming` (#56).

[Full changelog](https://github.com/run-ai/karta/compare/v0.0.12...v0.1.0)

## v0.0.1 through v0.0.12 - 2025-09-03 to 2026-04-14

Pre-rename development releases under the earlier project name. See the [release list](https://github.com/run-ai/karta/releases) for individual notes.
