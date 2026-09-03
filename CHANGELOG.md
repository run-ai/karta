<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Changelog - v0.1

Every release on the `v0.1` line: the `v0.1.0` minor and the patch releases tagged from this branch. Each entry carries the content of the published [GitHub Release](https://github.com/run-ai/karta/releases), normalized to the repository Markdown rules in [AGENTS.md](AGENTS.md).

`main`'s changelog covers minor and major releases across all lines.

## v0.1.1 - 2026-06-01

### What's Changed
* Bump Go version to 1.26.3 and update dependencies by @yuval-gr in https://github.com/run-ai/karta/pull/82


Full Changelog: https://github.com/run-ai/karta/compare/v0.1.0...v0.1.1

## v0.1.0 - 2026-05-12

This release completes the project rename from `ri` / `krt` to karta, decouples the library from `sigs.k8s.io/controller-runtime`, and introduces native suspend / resume support on the CRD. See [Breaking changes](#%EF%B8%8F-breaking-changes) for migration steps.

---

### Highlights

| | |
|---|---|
| Project rename to `karta` | API group is now `run.ai/v1alpha1`, the CRD `Kind` is `Karta`, and the Helm chart is named `karta`. ([#42](https://github.com/run-ai/karta/pull/42), [#50](https://github.com/run-ai/karta/pull/50)) |
| Zero controller-runtime dependency | karta no longer imports `sigs.k8s.io/controller-runtime`. Consumers that only need the CRD types or the resource helpers no longer inherit it transitively. ([#62](https://github.com/run-ai/karta/pull/62)) |
| Suspend / Resume support | New `SuspendDefinition` field on the CRD, and new resource statuses `Suspended`, `Suspending`, `Resuming`. ([#56](https://github.com/run-ai/karta/pull/56)) |

---

### Install

Helm chart (from GHCR):

```bash
helm install karta oci://ghcr.io/run-ai/karta/karta --version 0.1.0
```

Go consumers:

```bash
go get github.com/run-ai/karta@v0.1.0
```

```go
import karta "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
```

---

### Compatibility

| | |
|---|---|
| Kubernetes | tested against v1.30 - v1.34 (`k8s.io/api v0.35.1`) |
| Go | 1.25.9 or later |
| Helm | 3.14 or later |

---

### Breaking changes

#### 1. Project rename: `ri` / `krt` -> `karta`

| was | now |
|---|---|
| CRD `Kind` `ResourceInterface` | `Karta` |
| API group `optimization.nvidia.com/v1alpha1` | `run.ai/v1alpha1` |
| Go package `github.com/run-ai/karta/pkg/api/optimization/v1alpha1` | `github.com/run-ai/karta/pkg/api/runai/v1alpha1` |
| Field on `ComponentFactory`: `ri *ResourceInterface` | `karta *Karta` |
| Helm chart `krt` | `karta` |

Cluster migration - existing CRDs need to be re-applied under the new GVK:

```bash
kubectl get resourceinterfaces.optimization.nvidia.com -o yaml | \
    sed 's|optimization.nvidia.com/v1alpha1|run.ai/v1alpha1|; s|kind: ResourceInterface|kind: Karta|' | \
    kubectl apply -f -

kubectl delete crd resourceinterfaces.optimization.nvidia.com
```

Go consumer migration:

```diff
- import ri "github.com/run-ai/karta/pkg/api/optimization/v1alpha1"
+ import karta "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

- var obj ri.ResourceInterface
+ var obj karta.Karta

- factory := resource.NewComponentFactoryFromObject(myRi, kubeObj)
+ factory := resource.NewComponentFactoryFromObject(myKarta, kubeObj)
```

#### 2. `sigs.k8s.io/controller-runtime` no longer a karta dependency

karta now uses `k8s.io/apimachinery` directly for the two things it previously needed from controller-runtime:

| was | now |
|---|---|
| `&scheme.Builder{GroupVersion:...}` from `controller-runtime/pkg/scheme` | `runtime.NewSchemeBuilder(addKnownTypes)` from `k8s.io/apimachinery/pkg/runtime` |
| `client.Object` from `controller-runtime/pkg/client` (in `NewComponentFactoryFromObject` and `GetResource`) | local `KubernetesObject` interface (`metav1.Object` + `runtime.Object`) |

Consumer impact: no source code changes required. Go matches interfaces structurally (by method set), so:

```go
import "sigs.k8s.io/controller-runtime/pkg/client"

var pod client.Object = &corev1.Pod{...}
factory := resource.NewComponentFactoryFromObject(myKarta, pod)  // still compiles
obj, _ := factory.GetResource()
var asClientObject client.Object = obj                            // still compiles
```

Cleanup opportunity: consumers that only had `sigs.k8s.io/controller-runtime` in their `go.mod` because karta dragged it in can now drop it:

```bash
go get sigs.k8s.io/controller-runtime@none  # if you don't otherwise use it
go mod tidy
```

This is the dominant pattern across major K8s OSS - see the same idiom in [kubevirt/api](https://github.com/kubevirt/api/blob/0a124c6271bf8ad3b2263b3b16b4544bae794ecd/core/v1/register.go#L85), [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator/blob/0bd09d4dae2c319b9b43eb1921afe869f123d3a8/pkg/apis/monitoring/v1/register.go#L35), [tektoncd/pipeline](https://github.com/tektoncd/pipeline/blob/a3305b1d9b31f743fc625485e5c88837a731bbd3/pkg/apis/pipeline/v1/register.go#L40), [cert-manager](https://github.com/cert-manager/cert-manager/blob/7c3e89bf397945fa0143e3836144fa150bcc55c7/pkg/apis/certmanager/v1/register.go#L36), [cluster-api](https://github.com/kubernetes-sigs/cluster-api/blob/93bb18264d28410997c248052da4d7c4dbad7b91/api/core/v1beta2/groupversion_info.go#L30), [knative/pkg `kmeta.Accessor`](https://github.com/knative/pkg/blob/df317a52d1121053934fddcfff0e1081cfae42c2/kmeta/accessor.go#L32), and [crossplane-runtime `resource.Object`](https://github.com/crossplane/crossplane-runtime/blob/a596e1f7563505485b8ab0a06c1c3dc2d21ecf90/pkg/resource/interfaces.go#L193).

---

### New features

- Suspend / Resume definitions and statuses - new `SuspendDefinition` on the CRD, plus new resource statuses `Suspended`, `Suspending`, `Resuming`. ([#56](https://github.com/run-ai/karta/pull/56) by @TomB30)
- `ValidateParsedJQ` is now exported - external consumers that already hold a compiled `*gojq.Query` can run the read-only / safe-expression check without re-parsing. ([#53](https://github.com/run-ai/karta/pull/53) by @AviadHayumi)
- Helm chart published to GHCR on every release - pull with `helm install karta oci://ghcr.io/run-ai/karta --version 0.1.0`. ([#27](https://github.com/run-ai/karta/pull/27) by @AviadHayumi)
- Chart version bump enforcement in CI - `ct lint` blocks PRs that change the chart without bumping `Chart.yaml` version. Matches the prometheus-operator / argo enforcement model. ([#48](https://github.com/run-ai/karta/pull/48) by @AviadHayumi)
- **`docs/examples/*.yaml` validated in CI** - example Kartas are exercised by CI so docs cant silently drift from the schema. ([#61](https://github.com/run-ai/karta/pull/61) by @Isan-Rivkin)
- Issue templates + code of conduct - bug / feature templates plus a contributor CoC. ([#38](https://github.com/run-ai/karta/pull/38) by @yuval-gr)
- Helm chart annotated with OCI source - `helm.sh/chart-source` annotation links chart back to the GHCR package page. ([#49](https://github.com/run-ai/karta/pull/49) by @AviadHayumi)

### Bug fixes

- Repaired `docs/examples/mpijob.yaml` and simplified the pytorch running mapping. ([#58](https://github.com/run-ai/karta/pull/58) by @lavianalon)
- Removed accidentally pushed `docs/ri-studio/` directory. ([#40](https://github.com/run-ai/karta/pull/40) by @ronlv10)
- Fixed formatting and validation in issue templates. ([#41](https://github.com/run-ai/karta/pull/41) by @yuval-gr)

### Maintenance

- Renamed `ri` -> `krt` across the codebase (first half of the rename to karta). ([#42](https://github.com/run-ai/karta/pull/42) by @AviadHayumi)
- Renamed Helm chart `krt` -> `karta` (finishing the rename). ([#50](https://github.com/run-ai/karta/pull/50) by @AviadHayumi)
- Bumped `golang.org/x/net` to v0.51.0 (security). ([#45](https://github.com/run-ai/karta/pull/45) by @yuval-gr)
- Updated Go toolchain to 1.25.9. ([#30](https://github.com/run-ai/karta/pull/30) by @yuval-gr)

---

### Dependencies

#### Added

_nothing_

#### Changed

- `golang.org/x/net` v0.39.0 -> v0.51.0
- Go toolchain 1.25.7 -> 1.25.9
- `k8s.io/api` v0.34.x -> v0.35.1
- `k8s.io/apimachinery` v0.34.x -> v0.35.1

#### Removed

- `sigs.k8s.io/controller-runtime` (previously v0.23.1) - dropped entirely
- `sigs.k8s.io/controller-runtime/pkg/client` (transitive) - no longer pulled in
- `sigs.k8s.io/controller-runtime/pkg/scheme` (transitive) - no longer pulled in

---

### Contributors

thanks to everyone who shipped this release:

[@AviadHayumi](https://github.com/AviadHayumi), [@Isan-Rivkin](https://github.com/Isan-Rivkin), [@TomB30](https://github.com/TomB30), [@lavianalon](https://github.com/lavianalon), [@ronlv10](https://github.com/ronlv10), [@yuval-gr](https://github.com/yuval-gr)

---

Full changelog: https://github.com/run-ai/karta/compare/v0.0.12...v0.1.0
Helm chart: `oci://ghcr.io/run-ai/karta:0.1.0`
