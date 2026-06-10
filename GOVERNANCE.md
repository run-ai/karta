# Karta Governance

This document describes how the Karta project is governed. It is a starting point
that will grow as the contributor community grows.

## Principles

Karta is run as an open, vendor-neutral project. We aim to:

- **Balance the interests of all stakeholders** — contributors, adopters, and the
  broader ecosystem — without favoring any single organization.
- **Keep the contract thin and neutral.** Karta describes workload *structure*; it
  does not schedule, queue, or own a workload's lifecycle. Keeping the core small
  is what lets any tool — from any vendor — depend on it.
- **Decide in the open.** Significant technical decisions are made through public
  issues, pull requests, and discussions, so that any contributor or adopter can
  follow and weigh in.

## Maintainers and decision-making

The current maintainers, their areas of responsibility, and affiliations are
listed in [MAINTAINERS.md](MAINTAINERS.md). Maintainers are responsible for
reviewing contributions, setting technical direction, and stewarding releases.

Decisions are made by lazy consensus among maintainers on the relevant issue or
pull request. Where consensus cannot be reached, maintainers resolve the matter by
a simple majority. No single organization is the deciding authority for the
project.

## Vendor neutrality

Karta originated at Run:ai (NVIDIA) but is intended as a community standard, not a
vendor-owned one. To keep it neutral:

- **No organization-specific gating.** Features are not withheld or paywalled to
  drive purchase of any commercial product. The API and the bundled examples stay
  generally useful to any consumer.
- **Neutral, stable consumption surface.** Karta is consumed both as a CRD and as a
  Go library. The API group and the library follow a documented versioning and
  deprecation policy so that any downstream project — regardless of vendor — can
  depend on Karta with predictable upgrade expectations. (This is why the roadmap
  prioritizes moving off a vendor-prefixed API group before the API stabilizes.)
- **Open contribution and review.** Contributors and adopters get a fair
  opportunity to reach consensus on features and PRs through the public review
  process, without one organization's priorities overriding the community's.

## Project home and evolution

Karta is committed to vendor-neutral governance. Over time the project intends to
move to a neutral, community-governed home (a cloud native foundation) once the API
group is vendor-neutral and the project demonstrates adoption across multiple
organizations. Until then it is developed in the open under its current
organization, accepting external contributions.

## Code of conduct

All participation in the project is governed by the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Changing this document

This governance document is itself maintained in the open. Proposed changes are
made via pull request and require maintainer approval, following the same process
as any other change to the project.
