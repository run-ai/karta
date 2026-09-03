<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Changelog - v0.2 patch releases

Patch releases on the `v0.2` line, tagged from this branch. Each entry is the published [GitHub Release](https://github.com/run-ai/karta/releases) body verbatim.

Minor and major releases are tagged from `main` and documented in the changelog there.


## v0.2.3 - 2026-08-17

Adds a digest-pinned air-gap image lock to the release. Each release now carries
per-platform `ImageLock` YAML assets (linux/amd64, linux/arm64) that list the exact
image digests needed to install Karta in an air-gapped cluster.

### What's Changed
* feat(release): generate air-gap image locks per release (v0.2 backport) by @AviadHayumi in #237

**Full Changelog**: https://github.com/run-ai/karta/compare/v0.2.2...v0.2.3

## v0.2.2 - 2026-07-23

### What's Changed
* Fix/v0.2 crd cache transform by @shaked-bouktus in https://github.com/run-ai/karta/pull/177


**Full Changelog**: https://github.com/run-ai/karta/compare/v0.2.1...v0.2.2

## v0.2.1 - 2026-07-20

### What's Changed
* fix(chart): bump operator memory limit to 256Mi by @shaked-bouktus in https://github.com/run-ai/karta/pull/164


**Full Changelog**: https://github.com/run-ai/karta/compare/v0.2.0...v0.2.1
