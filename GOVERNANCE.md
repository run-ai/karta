# Karta Governance

This document describes how the Karta project is governed. It is a starting point
that will grow as the contributor community grows.

## Principles

Karta is run as an open, vendor-neutral project. The project aims to:

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

## Becoming a maintainer

Maintainership is earned through sustained, high-quality contribution, and the
ladder is open to contributors from any organization.

- **Contributor** - anyone who opens issues or pull requests. No prior status is
  required; see [CONTRIBUTING.md](CONTRIBUTING.md) to get started.
- **Reviewer** - a contributor with a track record of quality contributions and
  review feedback in an area of the project. Reviewers are expected to help triage
  issues and review pull requests in their area.
- **Maintainer** - a reviewer who has demonstrated sustained ownership and sound
  technical judgment. Maintainers carry merge rights and share responsibility for
  direction, reviews, and releases.

A contributor is nominated for reviewer or maintainer by an existing maintainer,
typically after a consistent history of merged contributions. The nomination is
made in a public pull request that adds the person to [MAINTAINERS.md](MAINTAINERS.md)
(and `OWNERS` where applicable), and is confirmed by lazy consensus of the current
maintainers. Promotion is based on contribution and judgment, not affiliation; the
project actively wants maintainers from more than one organization.

## Stepping down and emeritus

Maintainers may step down at any time by opening a pull request that removes them
from [MAINTAINERS.md](MAINTAINERS.md) and `OWNERS`. Maintainers who are no longer
active may be moved to emeritus status: they are recognized for their past
contributions and are welcome to return, but they no longer carry active
maintainer responsibilities or merge rights.

A maintainer who is inactive for an extended period, or who is unable to uphold
these responsibilities, may be moved to emeritus by lazy consensus of the
remaining maintainers via a public pull request. The intent is to keep the active
maintainer list an accurate reflection of who is currently stewarding the project,
not to penalize anyone.

## Vendor neutrality

Karta originated at Run:ai (NVIDIA) but is intended as a community standard, not a
vendor-owned one. To keep it neutral:

- **No organization-specific gating.** Features are not withheld or paywalled to
  drive purchase of any commercial product. The API and the bundled examples stay
  generally useful to any consumer.
- **Neutral, stable consumption surface.** Karta is consumed both as a CRD and as a
  Go library. The API group and the library follow a documented versioning and
  deprecation policy so that any downstream project — regardless of vendor — can
  depend on Karta with predictable upgrade expectations.
- **Open contribution and review.** Contributors and adopters get a fair
  opportunity to reach consensus on features and PRs through the public review
  process, without one organization's priorities overriding the community's.

## Project home and evolution

Karta is committed to vendor-neutral governance. Over time the project intends to
move to a neutral, community-governed home (a cloud-native foundation) once the API
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
