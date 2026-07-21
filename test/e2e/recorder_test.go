// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/conformance"
)

// stateAction mutates the live workload once its state is reached, so a flow can
// drive a transition the operator will not make on its own (for example resuming a
// suspended workload). A flow maps a state name to its action; it runs in both the
// recording and non-recording paths, and only once per state.
type stateAction func(ctx context.Context, obj *unstructured.Unstructured) error

// namedState pairs a state name with the predicate that recognises it from the
// workload's own status. States are declared in the order the workload passes
// through them; the last one is terminal and the recording waits for it.
type namedState struct {
	name  string
	ready readyFunc
}

// recordEnabled reports whether this run should write conformance fixtures.
// Recording is opt-in (make record-e2e) so a plain make test-e2e still only
// asserts and writes nothing.
func recordEnabled() bool {
	return os.Getenv("KARTA_RECORD") == "1"
}

// captured is one distinct state the workload passed through: the raw CR and the
// named state it classified as (empty for a state that has no declared predicate).
type captured struct {
	state string
	raw   *unstructured.Unstructured
}

// recording holds every distinct state the workload passed through, deduplicated by
// sanitized content so pure resourceVersion churn is dropped but any real status
// change is kept, in observation order. order is the sequence of named states, for
// the ordering check.
type recording struct {
	order     []string
	snapshots []captured
}

// classify returns the most advanced state whose predicate matches u (the last in
// declaration order). It judges the state from the workload's own fields, never
// from Karta, so the recorded expected output is never compared against itself.
func classify(u *unstructured.Unstructured, states []namedState) string {
	name := ""
	for _, s := range states {
		if s.ready(u) {
			name = s.name
		}
	}
	return name
}

// statusSettled reports whether the CR's status has caught up to its spec. When
// both metadata.generation and status.observedGeneration are present, the status is
// only trusted once observedGeneration >= generation, so a mid-reconcile snapshot is
// never recorded. CRs without those fields (for example batch/v1 Jobs) are always
// settled and are gated by their own conditions instead.
func statusSettled(u *unstructured.Unstructured) bool {
	gen, hasGen, _ := unstructured.NestedInt64(u.Object, "metadata", "generation")
	obs, hasObs, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	if !hasGen || !hasObs {
		return true
	}
	return obs >= gen
}

// sanitizedKey renders u's sanitized content as a stable string. observeTransitions
// deduplicates snapshots by this key, so it drops the volatile churn a re-record would
// strip anyway (resourceVersion, timestamps) while keeping every real status change -
// exactly the granularity golden needs to catch a reading regression on any state.
func sanitizedKey(u *unstructured.Unstructured) string {
	c := u.DeepCopy()
	conformance.Sanitize(c)
	b, err := json.Marshal(c.Object) // Go sorts map keys, so this is deterministic
	if err != nil {
		return u.GetResourceVersion() // fall back to no dedup on the rare marshal error
	}
	return string(b)
}

// dumpStatus renders a CR's status as indented JSON for a failure message, so a
// capture timeout is debuggable from the logs alone.
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

// observeTransitions watches the workload and captures every distinct settled CR it
// passes through (deduplicated by sanitized content), in order, until it reaches the
// terminal state or timeout. It uses a RetryWatcher so a watch that the API server
// expires mid-recording resumes from the last resourceVersion instead of busy-spinning
// on a closed channel.
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
	seenContent := map[string]bool{}
	actioned := map[string]bool{}
	var lastSeen *unstructured.Unstructured
	terminal := fl.states[len(fl.states)-1].name

	// Some workloads reach their terminal state at creation and the controller never
	// updates them afterwards (an unfired CronJob keeps an empty status), so a watch
	// from the creation resourceVersion would deliver no event to classify. Capture
	// the object as created when it is already terminal. The create response is
	// pre-reconcile, so it is deterministic across re-records.
	if statusSettled(obj) && classify(obj, fl.states) == terminal {
		rec.snapshots = append(rec.snapshots, captured{state: terminal, raw: obj.DeepCopy()})
		rec.order = append(rec.order, terminal)
		return rec
	}

	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			Fail(fmt.Sprintf("workload %s flow %q did not reach %q within %s; observed %v\nlast-seen status:\n%s",
				tc.name, fl.name, terminal, timeout, rec.order, dumpStatus(lastSeen)))
		case event, open := <-watcher.ResultChan():
			if !open {
				Fail("watch closed before terminal state")
			}
			u, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue // a bookmark or error frame carries no workload object
			}
			lastSeen = u
			state := classify(u, fl.states)
			// Capture every DISTINCT settled CR the workload passes through,
			// deduplicated by sanitized content. Pure resourceVersion churn is dropped,
			// but any real status change is its own snapshot even when it classifies to
			// the same state as its neighbour: Running -> Running -> Running with
			// different underlying fields is three snapshots. Golden replays EVERY
			// snapshot, so a library change that alters the reading of an intermediate
			// state - a middle "Running" a change would read Degraded - is caught, not
			// only a change at a state boundary. Statuses that have not caught up to the
			// spec (observedGeneration < generation) are skipped so a mid-reconcile CR is
			// never taken; undeclared transitions (state == "") are skipped so a snapshot
			// always carries a state to replay against.
			if statusSettled(u) && state != "" {
				if key := sanitizedKey(u); !seenContent[key] {
					seenContent[key] = true
					rec.snapshots = append(rec.snapshots, captured{state: state, raw: u.DeepCopy()})
					rec.order = append(rec.order, state)
				}
				// Fire the state's action once (for example unsuspend a suspended
				// workload), so the recording can capture the transition it triggers.
				if a := fl.actions[state]; a != nil && !actioned[state] {
					actioned[state] = true
					Expect(a(ctx, obj)).NotTo(HaveOccurred(), "action for state %q", state)
				}
				if state == terminal {
					return rec
				}
			}
		}
	}
}

// assertObservedOrder checks the states the workload was observed passing through are
// a monotonic subsequence of the flow's declared states, ending at the terminal. The
// workload may sit in one state across several distinct CRs (recorded as repeated
// snapshots), so consecutive repeats are collapsed first; a fast workload may also skip
// an intermediate, so a subsequence (not equality) still catches an undeclared state, an
// out-of-order regression, or a wrong terminal.
func assertObservedOrder(fl flow, order []string) {
	seq := conformance.CollapseConsecutive(order)
	idx := map[string]int{}
	for i, s := range fl.states {
		idx[s.name] = i
	}
	prev := -1
	for _, name := range seq {
		i, ok := idx[name]
		Expect(ok).To(BeTrue(), "observed undeclared state %q", name)
		Expect(i).To(BeNumerically(">", prev), "state %q observed out of declared order in %v", name, seq)
		prev = i
	}
	Expect(seq).NotTo(BeEmpty(), "no states observed")
	Expect(seq[len(seq)-1]).To(Equal(fl.states[len(fl.states)-1].name), "terminal must be the last observed state")
}

// writeFixture projects each captured snapshot through the Karta library and writes
// the fixture under test/conformance/fixtures/<operator>/<version>/<kartaName>/. It runs
// only after the live assertion passed, so nothing is recorded from a failing or flaky run.
func writeFixture(tc workloadCase, fl flow, karta *kartav1alpha1.Karta, rec *recording) {
	// assertObservedOrder already ran in the harness (both record and non-record
	// paths call it), so the order is verified before we get here.
	version := operatorVersion(tc.operator)

	fixture := conformance.Fixture{
		SchemaVersion:  conformance.SchemaVersion,
		Operator:       tc.operator,
		Version:        version,
		KartaName:      tc.kartaName,
		Flow:           fl.name,
		Want:           fl.want,
		KartaFile:      strings.TrimPrefix(tc.kartaFile, "../../"),
		ObservedStates: conformance.CollapseConsecutive(rec.order),
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

		dir := conformance.SnapshotDir(i, snap.state)
		fixture.Snapshots = append(fixture.Snapshots, conformance.Snapshot{State: snap.state, Dir: dir})
		data[dir] = conformance.SnapshotData{CR: sanitized, Expected: reading}
	}

	root := filepath.Join("..", "conformance", "fixtures", tc.operator, version, tc.kartaName, fl.name)
	Expect(conformance.Write(root, fixture, data)).To(Succeed())
	GinkgoWriter.Printf("recorded test/conformance/fixtures/%s/%s/%s/%s (%d snapshots, named states %v)\n", tc.operator, version, tc.kartaName, fl.name, len(rec.snapshots), rec.order)
}

// operatorVersion returns the version hack/e2e/up.sh actually installed for op,
// read from the .installed-versions file it writes; "unknown" when it is absent.
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
