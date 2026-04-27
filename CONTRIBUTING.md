# Contributing to karta

Thank you for your interest in contributing to karta! This document provides guidelines and instructions for contributing to this project.

## Developer Certificate of Origin (DCO)

All contributions to this project must comply with the Developer Certificate of Origin (DCO) version 1.1. This is a lightweight way for contributors to certify that they wrote or otherwise have the right to submit the code they are contributing to the project.

### DCO Sign-off

By contributing to this project, you agree to the DCO. You must sign off your commits to indicate that you agree to the DCO. You can do this by adding a `Signed-off-by` line to your commit messages:

```text
Signed-off-by: Your Name <your.email@example.com>
```

You can sign off automatically by using the `-s` or `--signoff` flag when committing:

```bash
git commit -s -m "Your commit message"
```

You must use your real name (sorry, no pseudonyms or anonymous contributions).

### DCO Text

The full text of the DCO version 1.1 is as follows:

```text
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

For more information about the DCO, please visit: https://developercertificate.org/

## Contribution Guidelines

### Before You Start

1. Check existing issues and pull requests to see if your contribution is already being addressed
2. Ensure your code follows the project's coding standards and conventions
3. Every pull request must reference at least one open GitHub issue in its description. This ensures that all changes are tracked and linked to project requirements or bug reports.
4. For major changes, please open an issue first to discuss the proposed changes



### Making Changes

1. Fork the repository
2. Create a feature branch from the main branch
3. Make your changes
4. Ensure all tests pass
5. Sign off your commits with the DCO (see above)
6. Submit a pull request

### Pull Request Process

1. Ensure your pull request includes:
   - A clear description of the changes
   - Reference to any related issues
   - All commits signed off with DCO
   - Tests for new functionality (if applicable)
   - Updated documentation (if applicable)

2. Your pull request will be reviewed by maintainers
3. Address any feedback or requested changes
4. Once approved, your changes will be merged

## Versioning

`charts/karta/Chart.yaml` carries two version fields. Each tracks a different concern and is bumped at a different point in the workflow.

| Field | Tracks | When to bump |
|---|---|---|
| `version` | Chart semver (RBAC, templates, values defaults, README) | every PR that changes files under `charts/karta/` (excluding `Chart.yaml` itself) |
| `appVersion` | Controller/CRD release (the code we ship) | as part of the release prep before tagging |

Both follow [semver](https://semver.org/). Pick the bump level (patch / minor / major) using the same rules you would for any semver release.

This convention follows the pattern used by [prometheus-community/helm-charts](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/Chart.yaml), [argoproj/argo-helm](https://github.com/argoproj/argo-helm/blob/main/charts/argo-cd/Chart.yaml), and [kubernetes/ingress-nginx](https://github.com/kubernetes/ingress-nginx/blob/main/charts/ingress-nginx/Chart.yaml).

### Day-to-day: chart changes bump `version`

Any PR that touches a file under `charts/karta/` (other than `Chart.yaml`) must bump `version` in `charts/karta/Chart.yaml`. Edit the file directly:

```yaml
# charts/karta/Chart.yaml
version: 0.1.1   # was 0.1.0
```

The [chart-lint workflow](.github/workflows/chart-lint.yaml) runs [`ct lint`](https://github.com/helm/chart-testing) (the official helm chart-testing tool) on every PR. With its default `--check-version-increment=true`, it fails the PR if a modified chart's `version` was not bumped. Same enforcement [prometheus-community/helm-charts](https://github.com/prometheus-community/helm-charts/blob/main/.github/workflows/lint-test.yaml) and [argoproj/argo-helm](https://github.com/argoproj/argo-helm/blob/main/.github/workflows/lint-and-test.yml) use.

### Code changes don't bump on every PR

PRs that change `pkg/` (or anything else outside `charts/karta/`) don't need to bump `version` or `appVersion`. The CI doesn't enforce one. `appVersion` stays at the last released value while code accumulates on main.

### Release prep: bump `appVersion` to match the next tag

When ready to release a new controller version (for example, `v1.2.3`):

1. Open a release-prep PR that bumps `appVersion` (and usually `version` too) in `charts/karta/Chart.yaml`:

   ```yaml
   version: 0.5.0
   appVersion: "1.2.3"
   ```

2. Merge the PR.
3. Tag the merge commit:

   ```bash
   git tag v1.2.3
   git push origin v1.2.3
   ```

4. The [push-artifacts workflow](.github/workflows/push-artifacts.yaml) validates `tag == appVersion` and publishes the chart. If the tag doesn't match `appVersion`, it fails loudly.

This is the same release-prep flow used by [openbao-operator](https://github.com/dc-tec/openbao-operator/blob/main/.github/workflows/release.yml) and [ingress-nginx](https://github.com/kubernetes/ingress-nginx).

## Code of Conduct

Please be respectful and professional in all interactions. We are committed to providing a welcoming and inclusive environment for all contributors.

## Questions?

If you have questions about contributing, please open an issue or contact the maintainers.

Thank you for contributing to karta!
