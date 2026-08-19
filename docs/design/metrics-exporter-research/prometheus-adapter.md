<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# prometheus-adapter: config-driven relabeling of per-pod series

## TL;DR

- Every metric is driven by a four-part config rule: discovery (seriesQuery), association (resources), naming (name), querying (metricsQuery). No code changes per metric (docs/config.md).
- Association is pure label mapping. A Prometheus label name is declared to mean a Kubernetes group-resource, either per label (overrides) or by a naming pattern (template). The label value becomes the object name (pkg/naming/resource_converter.go).
- The adapter never stores or re-emits samples. It keeps only a periodically refreshed cache of series names, and translates each API call into a fresh PromQL query on demand (pkg/custom-provider/provider.go).
- Aggregation to object granularity is delegated to Prometheus. The rule's metricsQuery template gets LabelMatchers and GroupBy filled in, so sum by (pod) or by any resource label runs server side (pkg/naming/metrics_query.go).
- The whole model breaks silently when labels do not match the config. The docs devote troubleshooting sections to exactly this failure (docs/walkthrough.md).

## How it works

Rule model. A config file holds a list of DiscoveryRule entries with fields SeriesQuery, SeriesFilters, Resources, Name, and MetricsQuery (pkg/config/config.go). Each rule is compiled into a MetricNamer that bundles a series selector, regex name rewriter, series filters, and a resource converter (pkg/naming/metric_namer.go, NamersFromConfig).

Discovery. A cachingMetricsLister runs on a timer (--metrics-relist-interval, default 10m per README.md). It sends each rule's seriesQuery to the Prometheus /api/v1/series endpoint, deduplicating identical selectors and querying in parallel (pkg/custom-provider/provider.go, updateMetrics). Only series with datapoints inside --metrics-max-age survive, so a relist interval shorter than the scrape interval makes metrics flap in and out of the API (README.md, docs/walkthrough.md troubleshooting).

Association. For each discovered series, ResourcesForSeries walks the series labels. A label maps to a Kubernetes group-resource either through an explicit override (kubernetes_pod_name -> pod) or by matching the template pattern (kube_<<.Group>>_<<.Resource>>). Results are cached in bidirectional label<->resource maps under a RWMutex, and group-resources are normalized through the RESTMapper so pod and pods both work (pkg/naming/resource_converter.go). A series is exposed once per associated resource, so one series with pod, namespace, and service labels becomes three API metrics (docs/config-walkthrough.md). Namespacing is inferred: anything not namespace, node, or persistentvolume marks the metric namespaced (resource_converter.go, ResourcesForSeries).

Naming. A regex on the Prometheus name plus a capture-group substitution produces the API name, for example ^(.*)_total -> ${1}_per_second (pkg/naming/metric_namer.go, MetricNameForSeries). The registry stores API-name -> original-series mappings so the reverse lookup works at query time (pkg/custom-provider/series_registry.go, SetSeries).

Querying. An API call arrives as (metric, group-resource, namespace, object names). The adapter builds label matchers from those inputs: namespace label from the converter, resource label matched with = for one name or =~ name1|name2 for many, and puts the resource label into GroupBy (pkg/naming/metrics_query.go, Build). These are injected into the rule's Go-template metricsQuery, typically sum(rate(<<.Series>>{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>). Prometheus executes it and the adapter matches result vectors back to object names by reading the resource label off each returned series (pkg/custom-provider/series_registry.go, MatchValuesToNames; pkg/custom-provider/provider.go, metricsFor). The container-level resource metrics path shows the extraGroupBy hook for sub-object granularity (pkg/resourceprovider/provider.go passes containerLabel as extraGroupBy).

Failure mode when labels do not match. If a returned series lacks the expected resource label, MatchValuesToNames maps it to the empty name and the object silently gets no value; GetMetricByName then returns NotFound (pkg/custom-provider/provider.go). If discovery labels do not match the overrides, the metric never appears in the API at all. The walkthrough explicitly warns that custom relabelling rules or labels other than namespace and pod require config edits (docs/walkthrough.md).

Delegation tradeoffs. No storage and no re-emission: values are always as fresh as Prometheus at request time, and there is no second copy of the data to size or retain. The costs: one live PromQL query per API request plus periodic series listings that "can take a while on large clusters" (comment in pkg/custom-provider/provider.go, updateMetrics), a hard runtime dependency on Prometheus availability, and only best-effort timestamps (provider.go metricFor has a TODO and stamps time.Now()). Label selector semantics also degrade: Kubernetes In/NotIn selectors are lowered to Prometheus regex matches, and Gt/Lt are rejected outright (pkg/naming/metrics_query.go, selectMatcher, operatorIsSupported; sentinel errors in pkg/naming/errors.go).

Cardinality and maintenance. Rules run independently and must be mutually exclusive; overlap is the operator's problem (docs/config.md, pkg/config/config.go comment). SeriesFilters exist purely to disambiguate overlapping seriesQuery results (docs/sample-config.yaml shows isNot: ^container_.*_seconds_total$ carving one rule out of another). Discovery cost is bounded by deduplicating identical seriesQuery strings into a single Prometheus call (pkg/custom-provider/provider.go). The registry itself only holds series names, never samples, which keeps adapter memory proportional to metric-name cardinality, not sample cardinality.

## Relevance to Karta

The adapter solves the same shape of problem the Karta Metrics Exporter faces: existing per-pod telemetry must be attributed to a higher-level object the consumer cares about. Its answer is a declarative mapping layer (label -> resource) plus server-side aggregation (group by the resource label), with zero new metric storage. Karta already owns the harder half of this mapping: a Karta object knows its components and pod selectors, so Karta can compute the pod -> component -> workload association from the API server instead of requiring users to hand-declare which Prometheus label means what. Where the adapter needs a config rule per label convention, Karta can join on pod name and namespace, which every sane per-pod series already carries.

## Evidence

- docs/config.md - four-part rule model (discovery, association, naming, querying), seriesQuery, resources overrides and template, metricsQuery template fields Series, LabelMatchers, GroupBy. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/docs/config.md
- docs/config-walkthrough.md - end-to-end example mapping kubernetes_namespace and kubernetes_pod_name labels to resources; one series exposed on every associated resource. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/docs/config-walkthrough.md
- docs/walkthrough.md - troubleshooting for label mismatch and for metrics flapping when relist interval is below the scrape interval. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/docs/walkthrough.md
- docs/sample-config.yaml - real rules including seriesFilters used to keep overlapping rules mutually exclusive. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/docs/sample-config.yaml
- README.md - --metrics-relist-interval and --metrics-max-age semantics and their interaction. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/README.md
- pkg/config/config.go - DiscoveryRule, ResourceMapping (Template plus Overrides), NameMapping, ResourceRules with ContainerLabel. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/pkg/config/config.go
- pkg/naming/resource_converter.go - bidirectional label<->group-resource maps, template extraction, RESTMapper normalization, namespaced inference. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/pkg/naming/resource_converter.go
- pkg/naming/metrics_query.go - PromQL construction: matchers, GroupBy on the resource label, selector-to-matcher lowering and unsupported operators. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/pkg/naming/metrics_query.go
- pkg/naming/metric_namer.go - per-rule compilation: series selector, regex rename, series filters. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/pkg/naming/metric_namer.go
- pkg/custom-provider/provider.go - on-demand query execution, periodic parallel series relisting with selector dedup, time.Now() timestamp TODO. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/pkg/custom-provider/provider.go
- pkg/custom-provider/series_registry.go - name-only series cache, SetSeries fan-out per resource, MatchValuesToNames reading the resource label off result vectors. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/pkg/custom-provider/series_registry.go
- pkg/resourceprovider/provider.go - extraGroupBy with a configured container label for sub-pod granularity. https://github.com/kubernetes-sigs/prometheus-adapter/blob/master/pkg/resourceprovider/provider.go

## Lessons for Karta

- Separate the four concerns the way the adapter does: what series to pick up, how to attribute them, what to call them, how to aggregate them. Even if Karta hardcodes attribution via its CRD, keeping discovery, naming, and aggregation as explicit configurable stages pays off.
- Push aggregation to the metrics backend when possible. sum by (workload, component) in PromQL (or at recording-rule level) is cheaper and more correct than re-aggregating scraped samples in the exporter.
- Attribution is a join on labels, and the join key is the weak point. Karta should standardize on namespace plus pod name as the join key and resolve pod -> component -> workload itself from pod selectors, rather than asking users to describe label conventions per metric source. This removes the adapter's largest operational burden.
- Cache only identity, not values. The adapter's registry holds series names and mappings; sample data never lives in the adapter. If Karta re-emits metrics it will hold values by necessity, but it should still keep the attribution index (pod -> workload/component) as the only long-lived state.
- Make staleness explicit. The relist-interval versus scrape-interval flapping is a documented footgun. If Karta caches pod-to-workload mappings, document the window during which a new pod's metrics are unattributed.
- Deduplicate backend queries. The adapter collapses identical seriesQuery strings into one call; Karta should batch or collapse per-workload queries the same way.

## What NOT to copy

- Per-metric regex rule authoring. The rules file is powerful but every new metric family needs a rule, rules must be manually kept mutually exclusive, and misconfiguration fails silently as a missing metric. Karta's CRD should make attribution automatic, with config reserved for exceptions.
- Silent no-match behavior. MatchValuesToNames drops unmatched series without a signal and GetMetricByName degrades to NotFound. Karta should emit a visible signal (a metric or condition) when per-pod series cannot be attributed to any component.
- Fabricated timestamps. The adapter stamps results with time.Now() instead of the sample time (marked TODO in provider.go). Karta should propagate real sample timestamps or clearly define its freshness contract.
- Pure on-demand querying if Karta expects many scrapers or dashboards. One PromQL query per API request is fine behind the HPA, but a Prometheus scrape endpoint hit by multiple collectors would multiply backend load; precomputation (recording rules or a short-lived cache) fits an exporter better.
- Lossy selector translation. Lowering set operators to regex alternation changes semantics at the edges. If Karta exposes filtering, define supported operators up front and reject the rest loudly, as the adapter at least does for Gt/Lt.
