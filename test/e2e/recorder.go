// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/conformance"
)

var (
	ctx       = context.Background()
	k8sClient client.Client
	dynClient dynamic.Interface
)

func emptyLike(src *unstructured.Unstructured) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(src.GroupVersionKind())
	return u
}

// observeTransitions watches one workload from creation and records every distinct settled CR
// it moves through, firing each journey action once when its state is reached, until the
// terminal state is reached with all actions fired (or the timeout).
func observeTransitions(tc workloadCase, fl flow, obj *unstructured.Unstructured, timeout time.Duration) *recording {
	gvk := obj.GroupVersionKind()
	mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	Expect(err).NotTo(HaveOccurred(), "rest mapping for %s", gvk)

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	// Watch from the workload's creation resourceVersion so no early state is missed.
	initialRV := obj.GetResourceVersion()
	if initialRV == "" {
		seed := emptyLike(obj)
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), seed)).To(Succeed())
		initialRV = seed.GetResourceVersion()
	}

	watcher, err := watchtools.NewRetryWatcherWithContext(ctx, initialRV, &cache.ListWatch{
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", obj.GetName()).String()
			return dynClient.Resource(mapping.Resource).Namespace(namespace).Watch(ctx, opts)
		},
	})
	Expect(err).NotTo(HaveOccurred())
	defer watcher.Stop()

	rec := &recording{}
	var pending []step // journey steps with an action, fired head-first in order
	for _, st := range fl.journey {
		if st.action != nil {
			pending = append(pending, st)
		}
	}
	done := func(state kartav1alpha1.ResourceStatus) bool { return state == fl.want() && len(pending) == 0 }
	var lastSeen *unstructured.Unstructured

	// A workload terminal at creation never fires a watch event; take the create response.
	if statusSettled(obj) && done(classify(obj, tc.states)) {
		rec.keep(obj, fl.want())
		return rec
	}

	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			Fail(fmt.Sprintf("workload %s flow %q did not finish within %s; observed %v, want %q, unfired %v\nlast-seen status:\n%s",
				tc.name, fl.name, timeout, rec.order, fl.want(), stepStates(pending), dumpStatus(lastSeen)))
		case event, open := <-watcher.ResultChan():
			if !open {
				Fail("watch closed before terminal state")
			}
			u, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue // a bookmark or error frame carries no workload object
			}
			lastSeen = u
			state := classify(u, tc.states)
			if !statusSettled(u) || state == "" {
				continue // mid-reconcile or unrecognised: nothing to replay against
			}
			rec.keep(u, state)
			if len(pending) > 0 && state == pending[0].state {
				Expect(pending[0].action(ctx, obj)).NotTo(HaveOccurred(), "action at state %q", state)
				pending = pending[1:]
			}
			if done(state) {
				return rec
			}
		}
	}
}

// recording is one watched run: the snapshots worth freezing and the order the states happened
// in. A snapshot is dropped only when identical to the PREVIOUS kept one (pure churn); a genuine
// return to an earlier state (A -> B -> A) is kept, so the order check can catch a backwards jump.
type recording struct {
	order     []kartav1alpha1.ResourceStatus
	snapshots []capture
	lastKey   string
}

type capture struct {
	state kartav1alpha1.ResourceStatus
	raw   *unstructured.Unstructured // raw, not sanitized: the sanitize proof needs the original
}

func (r *recording) keep(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) {
	if key := sanitizedKey(u); key != r.lastKey {
		r.lastKey = key
		r.snapshots = append(r.snapshots, capture{state: state, raw: u.DeepCopy()})
		r.order = append(r.order, state)
	}
}

// classify returns the furthest-along state the workload matches, judged from its own fields.
func classify(u *unstructured.Unstructured, states []namedState) kartav1alpha1.ResourceStatus {
	var name kartav1alpha1.ResourceStatus
	for _, s := range states {
		if s.ready(u) {
			name = s.name
		}
	}
	return name
}

func stepStates(sts []step) []kartav1alpha1.ResourceStatus {
	out := make([]kartav1alpha1.ResourceStatus, len(sts))
	for i, st := range sts {
		out[i] = st.state
	}
	return out
}

// statusSettled reports whether the controller has caught up (observedGeneration >= generation);
// workloads without those fields count as settled.
func statusSettled(u *unstructured.Unstructured) bool {
	gen, hasGen, _ := unstructured.NestedInt64(u.Object, "metadata", "generation")
	obs, hasObs, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	if !hasGen || !hasObs {
		return true
	}
	return obs >= gen
}

// sanitizedKey is a workload's sanitized content as a stable string, so keep can tell a real
// state change from pure resourceVersion churn.
func sanitizedKey(u *unstructured.Unstructured) string {
	c := u.DeepCopy()
	conformance.Sanitize(c)
	b, err := json.Marshal(c.Object) // Go sorts map keys, so this is deterministic
	if err != nil {
		return u.GetResourceVersion()
	}
	return string(b)
}

func assertObservedOrder(fl flow, order []kartav1alpha1.ResourceStatus) {
	Expect(observedOrderErr(fl, order)).To(Succeed())
}

// observedOrderErr checks the observed states are an in-order subsequence of the journey, ending
// on the terminal. A fast workload may skip a step (subsequence, not equality), but a backwards
// jump or undeclared state is a regression. mayGoBackwards drops the ordering and only requires
// every observed state is in the journey.
func observedOrderErr(fl flow, order []kartav1alpha1.ResourceStatus) error {
	seq := slices.Compact(slices.Clone(order))
	if len(seq) == 0 {
		return fmt.Errorf("no states observed")
	}
	declared := stepStates(fl.journey)
	if fl.mayGoBackwards {
		for _, s := range seq {
			if !slices.Contains(declared, s) {
				return fmt.Errorf("observed undeclared state %q", s)
			}
		}
	} else {
		j := 0
		for _, s := range seq {
			for j < len(declared) && declared[j] != s {
				j++
			}
			if j == len(declared) {
				if slices.Contains(declared, s) {
					return fmt.Errorf("state %q observed out of journey %v", s, declared)
				}
				return fmt.Errorf("observed undeclared state %q; journey is %v", s, declared)
			}
			j++
		}
	}
	if last := seq[len(seq)-1]; last != fl.want() {
		return fmt.Errorf("terminal must be %q, observed %q", fl.want(), last)
	}
	return nil
}

// assertKartaTransitions checks Karta reads each recorded state the way the workload's own fields
// did, not just the terminal, so an intermediate mapping bug fails live.
func assertKartaTransitions(karta *kartav1alpha1.Karta, rec *recording) {
	for _, snap := range rec.snapshots {
		reading, err := conformance.Replay(karta, snap.raw)
		Expect(err).NotTo(HaveOccurred(), "replay state %q", snap.state)
		Expect(reading.MatchedStatuses).To(ContainElement(snap.state),
			"Karta should read %q here but matched %v", snap.state, reading.MatchedStatuses)
	}
}

// writeFixture replays each snapshot through Karta and writes the fixture under
// test/conformance/fixtures/. It runs only after the live assertions passed.
func writeFixture(tc workloadCase, fl flow, karta *kartav1alpha1.Karta, rec *recording) {
	version := operatorVersion(tc.operator)
	observed := slices.Compact(slices.Clone(rec.order))
	obs := make([]string, len(observed))
	for i, s := range observed {
		obs[i] = string(s)
	}

	fixture := conformance.Fixture{
		SchemaVersion:  conformance.SchemaVersion,
		Operator:       tc.operator,
		Version:        version,
		KartaName:      tc.kartaName,
		Flow:           fl.name,
		Want:           fl.want(),
		KartaFile:      strings.TrimPrefix(tc.kartaFile, "../../"),
		ObservedStates: obs,
	}
	data := map[string]conformance.SnapshotData{}
	for i, snap := range rec.snapshots {
		sanitized := snap.raw.DeepCopy()
		conformance.Sanitize(sanitized)

		rawReading, err := conformance.Replay(karta, snap.raw)
		Expect(err).NotTo(HaveOccurred())
		reading, err := conformance.Replay(karta, sanitized)
		Expect(err).NotTo(HaveOccurred())
		Expect(reading).To(Equal(rawReading), "sanitising snapshot %d changed what Karta reads", i)

		dir := conformance.SnapshotDir(i, string(snap.state))
		fixture.Snapshots = append(fixture.Snapshots, conformance.Snapshot{State: string(snap.state), Dir: dir})
		data[dir] = conformance.SnapshotData{CR: sanitized, Expected: reading}
	}

	root := filepath.Join("..", "conformance", "fixtures", tc.operator, version, tc.kartaName, fl.name)
	Expect(conformance.Write(root, fixture, data)).To(Succeed())
	GinkgoWriter.Printf("recorded %s/%s/%s/%s (%d snapshots %v)\n", tc.operator, version, tc.kartaName, fl.name, len(rec.snapshots), observed)
}

// operatorVersion is the version hack/e2e/up.sh installed for op, or "unknown".
func operatorVersion(op string) string {
	b, err := os.ReadFile(filepath.Join("..", "..", "hack", "e2e", "operators", ".installed-versions"))
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == op {
			return strings.TrimSpace(v)
		}
	}
	return "unknown"
}

// unsuspend clears spec.suspend so a suspended workload resumes. A merge patch, not a
// read-modify-write Update, so it does not race the controller reconciling a just-created
// workload, which would make an Update conflict on the resourceVersion.
func unsuspend(ctx context.Context, obj *unstructured.Unstructured) error {
	target := emptyLike(obj)
	target.SetName(obj.GetName())
	target.SetNamespace(obj.GetNamespace())
	return k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"suspend":false}}`)))
}

// dumpStatus renders a CR's status for a timeout failure message.
func dumpStatus(u *unstructured.Unstructured) string {
	if u == nil {
		return "(no object observed)"
	}
	status, _, _ := unstructured.NestedMap(u.Object, "status")
	b, err := json.MarshalIndent(status, "  ", "  ")
	if err != nil {
		return fmt.Sprintf("(status marshal error: %v)", err)
	}
	return "  " + string(b)
}

// recordEnabled gates fixture writing to make record-e2e (KARTA_RECORD=1).
func recordEnabled() bool {
	return os.Getenv("KARTA_RECORD") == "1"
}
