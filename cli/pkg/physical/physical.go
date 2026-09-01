// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package physical resolves the physical half of a workload tree: the nodes a
// component's pods landed on, and, where Dynamic Resource Allocation is in
// play, the individual devices those pods hold.
//
// The logical half of the tree (workload -> component -> pod) already comes
// out of pkg/tree via the Karta definition. This package completes the
// traversal to workload -> component -> pod -> node -> device, so the CLI can
// answer "which GPU is this rank on, and is it healthy" rather than just "how
// many GPUs did this rank ask for".
//
// Two tiers, deliberately separated because they have very different
// prerequisites:
//
//	Tier 1 (node): works on every cluster. Reads the Node objects the pods are
//	already bound to and reports readiness, cordon state, and topology domain.
//
//	Tier 2 (device): requires DRA. Reads the ResourceClaims named in
//	pod.status.resourceClaimStatuses and pulls the allocated device identity
//	out of the claim's allocation result. On the classic device-plugin path a
//	pod only carries a count ("nvidia.com/gpu: 8"), so device identity is not
//	recoverable and this tier stays empty.
//
// Every read here is best-effort. The CLI runs under the user's own
// kubeconfig, which frequently cannot read cluster-scoped Nodes, so a
// permission error degrades to a warning and a partial snapshot rather than a
// failed command.
package physical

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// DefaultTopologyLabels are the node labels consulted, in order, to name the
// topology domain a node sits in. The first one present wins.
//
// The NVIDIA clique label identifies an NVLink / NVL72 domain, which is the
// one that matters for gang placement: a tensor-parallel group split across
// two cliques is a performance bug that no logical view can surface. The
// standard zone label is the generic fallback.
var DefaultTopologyLabels = []string{
	"nvidia.com/gpu.clique",
	"topology.kubernetes.io/zone",
}

// Options tunes a resolution pass.
type Options struct {
	// TopologyLabels overrides DefaultTopologyLabels when non-empty.
	TopologyLabels []string
}

func (o Options) topologyLabels() []string {
	if len(o.TopologyLabels) > 0 {
		return o.TopologyLabels
	}
	return DefaultTopologyLabels
}

var (
	nodesGVR        = schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
	resourceClaimGK = schema.GroupKind{Group: "resource.k8s.io", Kind: "ResourceClaim"}
)

// NodeFacts is the Tier 1 payload: what we know about one node.
type NodeFacts struct {
	Name string
	// Ready mirrors the node's Ready condition. False covers both an explicit
	// NotReady and an Unknown status (a node whose kubelet stopped reporting).
	Ready bool
	// Unschedulable is the cordon flag, set during drains and maintenance.
	Unschedulable bool
	// Domain is the topology domain from the first matching TopologyLabels
	// entry, empty when the node carries none of them.
	Domain string
	// DomainLabel records which label Domain came from, for display.
	DomainLabel string
}

// Healthy reports whether the node is in a state a running workload can rely
// on. A cordoned node still runs its existing pods, but it signals in-flight
// maintenance, so it counts as degraded for a workload's purposes.
func (n NodeFacts) Healthy() bool { return n.Ready && !n.Unschedulable }

// Condition returns a short human label for an unhealthy node, or "" when the
// node is fine. NotReady wins over cordoned when both apply, since it is the
// more severe of the two.
func (n NodeFacts) Condition() string {
	switch {
	case !n.Ready:
		return "NotReady"
	case n.Unschedulable:
		return "cordoned"
	default:
		return ""
	}
}

// Device is one allocated DRA device, as reported by a ResourceClaim's
// allocation result.
type Device struct {
	Driver string
	Pool   string
	Name   string
}

func (d Device) String() string { return d.Name }

// Snapshot is the resolved physical view for one set of pods.
type Snapshot struct {
	// Nodes is keyed by node name.
	Nodes map[string]NodeFacts
	// Devices is keyed by "<namespace>/<podName>".
	Devices map[string][]Device
	// Warnings records best-effort reads that failed. The command surfaces
	// these on stderr; they never abort rendering.
	Warnings []string
	// DRAAvailable reports whether the cluster serves resource.k8s.io at all,
	// so the caller can tell "no devices allocated" apart from "no DRA here".
	DRAAvailable bool
}

// PodKey builds the Devices map key for a pod.
func PodKey(namespace, name string) string { return namespace + "/" + name }

// DevicesFor returns the devices allocated to a pod, or nil.
func (s *Snapshot) DevicesFor(namespace, name string) []Device {
	if s == nil {
		return nil
	}
	return s.Devices[PodKey(namespace, name)]
}

// NodeFor returns the facts for a node, and whether they were resolved.
func (s *Snapshot) NodeFor(name string) (NodeFacts, bool) {
	if s == nil || name == "" {
		return NodeFacts{}, false
	}
	f, ok := s.Nodes[name]
	return f, ok
}

// Resolve builds a Snapshot for the given pods. It performs one Get per
// distinct node and, when DRA is served, one List of ResourceClaims in the
// namespace.
//
// Errors are folded into Snapshot.Warnings rather than returned: a partial
// physical view is more useful than none, and the most common failure by far
// is simply not having cluster-scoped node read permission.
func Resolve(
	ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper, pods []corev1.Pod, opts Options,
) *Snapshot {
	snap := &Snapshot{Nodes: map[string]NodeFacts{}, Devices: map[string][]Device{}}
	if len(pods) == 0 {
		return snap
	}

	resolveNodes(ctx, dyn, pods, snap, opts)
	resolveDevices(ctx, dyn, mapper, pods, snap)
	return snap
}

// resolveNodes fetches the distinct nodes the pods are bound to. Unscheduled
// pods (empty NodeName) contribute nothing.
func resolveNodes(ctx context.Context, dyn dynamic.Interface, pods []corev1.Pod, snap *Snapshot, opts Options) {
	names := map[string]struct{}{}
	for i := range pods {
		if n := pods[i].Spec.NodeName; n != "" {
			names[n] = struct{}{}
		}
	}
	if len(names) == 0 {
		return
	}

	// One Get per distinct node rather than a cluster-wide List: a workload
	// spans a bounded node set, while a List scales with the cluster and is
	// frequently the read a user is not allowed to make.
	forbidden := false
	for _, name := range sortedKeys(names) {
		obj, err := dyn.Resource(nodesGVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsForbidden(err) {
				if !forbidden {
					forbidden = true
					snap.Warnings = append(snap.Warnings,
						"not permitted to read nodes; node health and topology omitted (needs get on nodes)")
				}
				continue
			}
			if apierrors.IsNotFound(err) {
				snap.Warnings = append(snap.Warnings, fmt.Sprintf("node %q not found", name))
				continue
			}
			snap.Warnings = append(snap.Warnings, fmt.Sprintf("read node %q: %v", name, err))
			continue
		}
		var node corev1.Node
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &node); err != nil {
			snap.Warnings = append(snap.Warnings, fmt.Sprintf("decode node %q: %v", name, err))
			continue
		}
		snap.Nodes[name] = nodeFacts(&node, opts.topologyLabels())
	}
}

// nodeFacts projects a Node object down to the handful of fields the tree
// renders.
func nodeFacts(node *corev1.Node, topologyLabels []string) NodeFacts {
	f := NodeFacts{Name: node.Name, Unschedulable: node.Spec.Unschedulable}
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			f.Ready = c.Status == corev1.ConditionTrue
			break
		}
	}
	for _, label := range topologyLabels {
		if v, ok := node.Labels[label]; ok && v != "" {
			f.Domain = v
			f.DomainLabel = label
			break
		}
	}
	return f
}

// resolveDevices maps pods to their allocated DRA devices.
//
// The hop that matters is pod.status.resourceClaimStatuses -> ResourceClaim
// -> status.allocation.devices.results. Without DRA there is no object
// anywhere in the API that says which physical device a pod holds, only how
// many it requested, which is exactly the gap this tier closes.
func resolveDevices(ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper, pods []corev1.Pod, snap *Snapshot) {
	wanted := map[string][]string{} // claim name -> pod keys
	namespace := ""
	for i := range pods {
		p := &pods[i]
		for _, cs := range p.Status.ResourceClaimStatuses {
			if cs.ResourceClaimName == nil || *cs.ResourceClaimName == "" {
				continue
			}
			namespace = p.Namespace
			key := PodKey(p.Namespace, p.Name)
			wanted[*cs.ResourceClaimName] = append(wanted[*cs.ResourceClaimName], key)
		}
	}

	// Resolve the served version of ResourceClaim through discovery. A
	// mapping error means the cluster does not serve DRA, which is simply the
	// classic device-plugin path, not a problem worth warning about.
	mapping, err := mapper.RESTMapping(resourceClaimGK)
	if err != nil {
		return
	}
	snap.DRAAvailable = true

	if len(wanted) == 0 {
		return
	}

	list, err := dyn.Resource(mapping.Resource).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			snap.Warnings = append(snap.Warnings,
				"not permitted to read resourceclaims; device identity omitted (needs list on resourceclaims)")
			return
		}
		snap.Warnings = append(snap.Warnings, fmt.Sprintf("list resourceclaims in %q: %v", namespace, err))
		return
	}

	for i := range list.Items {
		claim := &list.Items[i]
		podKeys, ok := wanted[claim.GetName()]
		if !ok {
			continue
		}
		devices := allocatedDevices(claim)
		if len(devices) == 0 {
			continue
		}
		for _, key := range podKeys {
			snap.Devices[key] = append(snap.Devices[key], devices...)
		}
	}

	for key := range snap.Devices {
		sort.Slice(snap.Devices[key], func(a, b int) bool {
			return snap.Devices[key][a].Name < snap.Devices[key][b].Name
		})
	}
}

// allocatedDevices reads status.allocation.devices.results out of a
// ResourceClaim. Unstructured access keeps this working across DRA API
// versions, whose Go types moved between v1beta1, v1beta2, and v1 while this
// field path stayed put.
func allocatedDevices(claim *unstructured.Unstructured) []Device {
	results, found, err := unstructured.NestedSlice(claim.Object, "status", "allocation", "devices", "results")
	if err != nil || !found {
		return nil
	}
	var out []Device
	for _, r := range results {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		d := Device{
			Driver: stringField(m, "driver"),
			Pool:   stringField(m, "pool"),
			Name:   stringField(m, "device"),
		}
		if d.Name == "" {
			continue
		}
		out = append(out, d)
	}
	return out
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FormatDevices renders a device list for a single tree row, capping the
// output so a 72-GPU rack does not wrap the terminal.
func FormatDevices(devices []Device, max int) string {
	if len(devices) == 0 {
		return ""
	}
	names := make([]string, 0, len(devices))
	for _, d := range devices {
		names = append(names, ShortDeviceName(d.Name))
	}
	if max > 0 && len(names) > max {
		extra := len(names) - max
		names = append(names[:max:max], fmt.Sprintf("+%d more", extra))
	}
	return strings.Join(names, ",")
}

// ShortDeviceName trims a device identifier for display. Real DRA drivers
// name devices by hardware UUID (gpu-58719de1-c4ad-5038-a163-c74eff36f8db),
// and a pod holding eight of them would run several hundred columns wide.
// The leading segment is enough to tell devices apart by eye; the full
// identifier stays on the ResourceClaim for anyone who needs to copy it.
func ShortDeviceName(name string) string {
	const keep = 8
	prefix, rest, found := strings.Cut(name, "-")
	if !found || len(rest) <= keep {
		return name
	}
	return prefix + "-" + rest[:keep] + "..."
}
