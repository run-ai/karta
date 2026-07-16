<!-- SPDX-License-Identifier: Apache-2.0 -->

<!-- Copyright (c) 2026 NVIDIA Corporation -->
# Karta Resource References - High-Level Design ## Background A Karta definition describes a workload by reading its manifest. Every value Karta extracts - pod spec, scale, status, pod selectors - is a JQ expression evaluated against a single root object: the workload CR itself. This works as long as everything Karta needs lives inside that one object.
It often does not.

Kubeflow Trainer v2 is the clearest example. A `TrainJob` points at a `ClusterTrainingRuntime` through `spec.runtimeRef`, and the runtime holds the base pod template - the containers and resources a job runs with. For example:

```yaml
# ClusterTrainingRuntime - the shared base every TrainJob of this kind runs with
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed
spec:
  template:               # a JobSet template
    spec:
      replicatedJobs:
        - name: node
          template:       # Job -> Pod template, trimmed to the container
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: pytorch/pytorch:2.3.0
                      resources:
                        limits: { nvidia.com/gpu: 1 }
---
# TrainJob - points at the runtime by name and overrides a few fields
kind: TrainJob
metadata:
  name: fine-tune
spec:
  runtimeRef:
    name: torch-distributed
  trainer:
    numNodes: 4
    resourcesPerNode:
      limits: { nvidia.com/gpu: 8 }
```

The `TrainJob` can override parts of it (nodes, resources, image, and so on), and those overrides take precedence over the runtime. The effective pod spec is therefore the combination of the runtime's template and the `TrainJob`'s overrides layered on top. A Karta definition for `TrainJob` can read the override fields on the `TrainJob` itself, but it has no way to reach the runtime, so it never sees the base it is overriding - it only ever has half of the spec. The same shape recurs elsewhere - a workload that points at a config object, a policy, or a dataset that holds part of the picture.

There is a second, related gap. Sometimes a value cannot be read from any single object and has to be computed from a set of them. Phase is the clearest case: some workloads do not report a usable status of their own, and the only way to tell whether one is running, degraded, or failed is to look across its pods and aggregate their states. To compute that, a definition needs the workload's pods as an input, not just the workload object.

We want to enrich a Karta definition by letting it draw on other cluster resources - a specific object or a set of them - in the same JQ expressions, in a generic way.
## Concept
A reference is a named pointer to another cluster resource, or set of resources, that Karta exposes to a definition's JQ expressions as the variable `$<Name>`. In this proposal, a Karta definition declares its references at the structure level. Each reference is resolved against the workload object itself (the root custom resource the definition describes, for example the `TrainJob`) - the expressions that pick the name, namespace, and labels run with that object as their input - and then fetched once from the cluster. Its result is in scope for the JQ expressions of every component. To the author it is just another input: where `.spec...` reads the workload object, `$trainingRuntime.spec...` reads a referenced one.

Declaring references once, for the whole workload, means the same value - the runtime, or the pod set - can be read by any number of components without being fetched again.

Karta stays declarative: it still fetches nothing itself. Resolving references is the consumer's job, and it adds one more input - the fetched values - to what the consumer passes when it evaluates a Karta definition.

A reference takes one of two shapes, declared in the API - `lookup` or `list` - and the shape decides what the variable holds:

- `lookup` - fetch a single object (a `Get`). `$<Name>` is that one object, read like any object (`$<name>.field`), or null if it does not exist.
  
- `list` - fetch a set of objects (a `List`). `$<Name>` is an array, read and iterated as `$<name>[]`, possibly empty.
  

Naming the two cases after what they return - one object versus many - keeps the variable's shape obvious to whoever writes the downstream expression.
## API
References are declared on the structure definition, as a list, alongside the root component, child components, and additional child kinds. Each entry names the variable, the GVK to fetch, and one of `lookup` or `list`. The list is keyed by the unique `name`.

A `lookup` names the single object to fetch: its `nameExpression` is a JQ expression against the root object that resolves the resource's name, and the consumer `Get`s that object (in the namespace from `namespaceExpression`, or the workload's own namespace).

A `list` selects its set with a structured selector in the shape of a Kubernetes `LabelSelector` (`matchLabels` plus `matchExpressions`), so it converts directly into a real selector for the `List` call. The one addition over a plain Kubernetes selector is that a value can be sourced from the root object (using JQ) rather than being a constant: most values are literals, but the workload's own name (for "pods of this workload") only exists at runtime. A value is therefore either a literal or a JQ expression evaluated against the root.

```go
type StructureDefinition struct {
    // ... RootComponent, ChildComponents, AdditionalChildKinds ...

    // References declares cluster resources whose values are exposed to the
    // workload's JQ expressions as the variable $<Name>.
    References []ResourceReference // +listType=map, +listMapKey=name
}

// ResourceReference declares another resource (or set of resources) whose values are
// exposed to every component's JQ expressions as the variable $<Name>.
// Exactly one of Lookup or List is set:
//   Lookup -> $<Name> is a single object (or null)
//   List   -> $<Name> is an array (possibly empty)
type ResourceReference struct {
    Name string           // variable name exposed to JQ as $<Name>
    GVK  GroupVersionKind // group/version/kind of the referenced resource(s)

    // NamespaceExpression is a JQ expression against the root object resolving the
    // namespace to fetch from. If empty, a namespaced GVK defaults to the root
    // object's namespace.
    NamespaceExpression *string

    Lookup *LookupReference
    List   *ListReference
}

// LookupReference fetches a single resource by name (consumer: client.Get).
type LookupReference struct {
    // NameExpression is a JQ expression against the root object resolving the
    // referenced resource's name.
    NameExpression string
}

// ListReference fetches a set of resources by a structured label selector
// (consumer: client.List). After value resolution it maps onto metav1.LabelSelector.
// At least one of MatchLabels or MatchExpressions must be set.
type ListReference struct {
    MatchLabels      map[string]LabelValue
    MatchExpressions []LabelSelectorRequirement
}

// LabelSelectorRequirement mirrors metav1.LabelSelectorRequirement, except its values
// may be sourced from the root object.
type LabelSelectorRequirement struct {
    Key      string
    Operator LabelSelectorOperator // In, NotIn, Exists, DoesNotExist
    Values   []LabelValue          // required for In/NotIn, empty for Exists/DoesNotExist
}

// LabelValue is a single label value: either a literal (Value) or a JQ expression
// evaluated against the root object (Expression). Value and Expression are a one-of;
// exactly one is set.
type LabelValue struct {
    Value      *string
    Expression *string
}
```

A definition reads as follows, with `references` a sibling of `rootComponent` under the structure definition. The first reference resolves the Kubeflow runtime; the second lists the workload's pods.

```yaml
spec:
  structureDefinition:
    rootComponent: { ... }
    childComponents: [ ... ]
    references:
      # single object: $trainingRuntime
      - name: trainingRuntime
        gvk: { group: trainer.kubeflow.org, version: v1alpha1, kind: ClusterTrainingRuntime }
        lookup:
          nameExpression: .spec.runtimeRef.name

      # set of objects: $pods
      - name: pods
        gvk: { group: "", version: v1, kind: Pod }
        namespaceExpression: .metadata.namespace
        list:
          matchLabels:
            training.kubeflow.org/job-name: { expression: .metadata.name }  # value from the root
            app.kubernetes.io/managed-by:   { value: karta }                # literal value
          matchExpressions:
            - key: training.kubeflow.org/replica-type
              operator: In
              values:
                - value: worker                          # literal
                - expression: .spec.primaryReplicaType   # value from the root
            - key: app.kubernetes.io/component
              operator: Exists
```

Any component's paths then read each variable by its shape - the variable is in scope for every component, so the two examples below can live on different components. A `lookup` is read like any object; a `list` is iterated.

```yaml
# the base pod template comes from the runtime; the TrainJob's overrides, read from
# the root object, are layered on top in the same expression:
podTemplateSpecPath: $trainingRuntime.spec.template.spec.replicatedJobs[0].template.spec.template
# an aggregate over the matched pods:
runningPodsPath: '[ $pods[] | select(.status.phase == "Running") ] | length'
```

The one-of rules - one of `lookup`/`list`, one of `value`/`expression`, and unique reference names - are enforced by Karta's existing validation function, alongside the JQ validation that already checks every expression field.
### Library API
References add a small, opt-in surface to the Karta library. A consumer that uses none of it is unaffected, and a definition with no `references` behaves exactly as today. Three pieces cover the feature.

`ResolvedReferences` is what a reference resolves to - an object for a `lookup`, a list for a `list`, keyed by name:

```go
// ReferenceValue is the fetched value of one reference.
type ReferenceValue struct {
    Object map[string]any   // set for a lookup
    List   []map[string]any // set for a list
}

// ResolvedReferences maps each reference name to its fetched value.
type ResolvedReferences map[string]ReferenceValue
```

`Resolve` fetches them. It lives in a new package, `pkg/references` - a new domain that owns reference resolution and is the one place in Karta that depends on a Kubernetes client; the types and `WithReferences` stay in the client-free `pkg/resource`. Given a reader, the Karta definition, and the workload object, it evaluates each reference's expressions against the workload, performs the `Get` (for a `lookup`) or `List` (for a `list`), and returns the map.

```go
func Resolve(ctx context.Context, reader client.Reader, karta *Karta, workload client.Object) (ResolvedReferences, error)
```

`WithReferences` binds them. When the component factory is built, the resolved references are passed as an option; each becomes the JQ variable `$<Name>`, in scope for every component's expressions.

```go
refs, err := references.Resolve(ctx, reader, karta, workload)
factory := resource.NewComponentFactoryFromObject(karta, workload, resource.WithReferences(refs))
```

If a definition declares a reference that is not provided to the factory, evaluating an expression that reads `$<Name>` errors instead of silently resolving to null.
## Caveats
A few points are worth calling out because they change the contract or carry real cost.

- References add a fetching step to the consumer. Until now a consumer could evaluate a Karta definition against an object it already held, with no cluster access. A definition that uses references requires a Kubernetes client and the RBAC to read the referenced kinds, and will not work with a consumer that has not implemented resolution. This is a capability the consumer opts into.
  
- Referencing arbitrary kinds could be expensive, depending on how the consumer fetches them and how many objects a `list` returns. Fetching through a cache can add further cost, so the consumer should choose its reader deliberately.
  
- A label value resolves to exactly one scalar. There is no fan-out from one expression into a variable-length set of values; authors enumerate the values they want. Sourcing a whole value set from the root (for example, every replica type a workload declares) is intentionally out of scope for the first version and can be added later without breaking the API.
  
- The reference uses `gvk` for its group/version/kind, while the existing `ComponentDefinition` names the same type `kind`. This inconsistency is deliberate for now - `kind.kind` reads poorly and `gvk` is accurate - but it should be resolved before the API stabilizes, either by renaming the existing field or accepting the divergence.
