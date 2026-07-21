// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// readyFunc reports whether a live workload object has reached the stable state
// the case asserts against. Assertions only ever target stable states, never a
// transient mid-state the upstream operator can pass through (see README).
type readyFunc func(*unstructured.Unstructured) bool

// condTrue matches when a status condition of the given type is True.
func condTrue(condType string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			m, ok := c.(map[string]any)
			if ok && m["type"] == condType && m["status"] == "True" {
				return true
			}
		}
		return false
	}
}

// condFalse matches when a status condition of the given type is present and False,
// for example a Deployment whose Progressing condition has flipped to False after
// exceeding its progress deadline.
func condFalse(condType string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			m, ok := c.(map[string]any)
			if ok && m["type"] == condType && m["status"] == "False" {
				return true
			}
		}
		return false
	}
}

// phaseEq matches when a string field at the given path equals want.
func phaseEq(want string, path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		got, _, _ := unstructured.NestedString(u.Object, path...)
		return got == want
	}
}

// phaseAny matches when a string field at the given path equals any of wants. Some
// operators map one Karta state from several phase strings (a DynamoGraphDeployment
// reads initializing from both "pending" and "initializing"), so a flow declares the
// intermediate state with all of them.
func phaseAny(wants []string, path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		got, _, _ := unstructured.NestedString(u.Object, path...)
		for _, w := range wants {
			if got == w {
				return true
			}
		}
		return false
	}
}

// intAtLeast matches when an integer field at the given path is present and >= min.
func intAtLeast(min int64, path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got >= min
	}
}

// replicasDegraded matches a replicated workload that has settled degraded: the
// controller has created every replica (updatedReplicas == replicas) but some are
// not ready (0 < readyReplicas < replicas). Requiring updatedReplicas == replicas
// excludes the transient mid-rollout where a StatefulSet has only brought up its
// first ordinals, so the assertion never races the workload into a false Degraded.
func replicasDegraded() readyFunc {
	return func(u *unstructured.Unstructured) bool {
		replicas, ok, _ := unstructured.NestedInt64(u.Object, "status", "replicas")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		updated, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
		return ok && replicas > 0 && updated == replicas && ready > 0 && ready < replicas
	}
}

// intBelow matches when an integer field at the given path is present and < max. It
// is the counterpart of intAtLeast for an initializing state (a Grove PodCliqueSet has
// availableReplicas present and below spec.replicas while its clique pods start). The
// field must be present, so the empty pre-reconcile status is not mistaken for it.
func intBelow(max int64, path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got < max
	}
}

// boolTrue matches when a boolean field at the given path is present and true.
func boolTrue(path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedBool(u.Object, path...)
		return err == nil && found && got
	}
}

// absent matches when the field at the given path is not present, for example a
// CronJob that has not scheduled yet (status.lastScheduleTime missing).
func absent(path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		_, found, _ := unstructured.NestedFieldNoCopy(u.Object, path...)
		return !found
	}
}

// cronjobFired matches a CronJob that has scheduled at least once
// (status.lastScheduleTime set) and whose spawned Job has finished
// (status.active empty), so the recorded snapshot carries no timestamped active-Job
// reference. Karta reads a fired, enabled CronJob as Running.
func cronjobFired() readyFunc {
	return func(u *unstructured.Unstructured) bool {
		_, scheduled, _ := unstructured.NestedFieldNoCopy(u.Object, "status", "lastScheduleTime")
		active, _, _ := unstructured.NestedSlice(u.Object, "status", "active")
		return scheduled && len(active) == 0
	}
}

// condsFalse matches when every listed status condition is present and False, for
// example the KServe failed mapping (PredictorReady, PredictorConfigurationReady, and
// RoutesReady all False).
func condsFalse(types ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, want := range types {
			ok := false
			for _, c := range conds {
				if m, is := c.(map[string]any); is && m["type"] == want && m["status"] == "False" {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}
		return len(types) > 0
	}
}

// jobsetRunning matches a JobSet with a running replicated job: some replicatedJob is
// ready and active with no failures (the sample's running expression).
func jobsetRunning() readyFunc {
	return func(u *unstructured.Unstructured) bool {
		rjs, _, _ := unstructured.NestedSlice(u.Object, "status", "replicatedJobsStatus")
		anyReadyActive := false
		for _, r := range rjs {
			m, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if failed, _, _ := unstructured.NestedInt64(m, "failed"); failed > 0 {
				return false
			}
			ready, _, _ := unstructured.NestedInt64(m, "ready")
			active, _, _ := unstructured.NestedInt64(m, "active")
			if ready > 0 && active > 0 {
				anyReadyActive = true
			}
		}
		return anyReadyActive
	}
}

// jobsetInitializing matches a JobSet whose pods are active but not yet ready: some
// replicatedJob has active > 0 and ready == 0, none failed. It mirrors the sample's
// initializing expression and is the ready==0 counterpart of jobsetRunning, so a
// readiness probe on the job pods yields Initializing before Running.
func jobsetInitializing() readyFunc {
	return func(u *unstructured.Unstructured) bool {
		rjs, _, _ := unstructured.NestedSlice(u.Object, "status", "replicatedJobsStatus")
		anyActiveNotReady := false
		for _, r := range rjs {
			m, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if failed, _, _ := unstructured.NestedInt64(m, "failed"); failed > 0 {
				return false
			}
			ready, _, _ := unstructured.NestedInt64(m, "ready")
			active, _, _ := unstructured.NestedInt64(m, "active")
			if active > 0 && ready == 0 {
				anyActiveNotReady = true
			}
		}
		return anyActiveNotReady
	}
}

// jobDegraded matches a parallel Job settled degraded: parallelism > 1, some but not
// all pods ready, and at least one pod already succeeded or failed - the Job analog of
// replicasDegraded.
func jobDegraded() readyFunc {
	return func(u *unstructured.Unstructured) bool {
		par, ok, _ := unstructured.NestedInt64(u.Object, "spec", "parallelism")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "ready")
		succeeded, _, _ := unstructured.NestedInt64(u.Object, "status", "succeeded")
		failed, _, _ := unstructured.NestedInt64(u.Object, "status", "failed")
		return ok && par > 1 && ready > 0 && ready < par && (succeeded > 0 || failed > 0)
	}
}

// raySuspended matches a RayCluster suspended at creation: spec.suspend is true and
// status.state is either "suspended" or not yet set (the sample's suspended expr,
// which also matches when the operator has not written a state yet).
func raySuspended() readyFunc {
	return func(u *unstructured.Unstructured) bool {
		if suspend, _, _ := unstructured.NestedBool(u.Object, "spec", "suspend"); !suspend {
			return false
		}
		state, found, _ := unstructured.NestedString(u.Object, "status", "state")
		return !found || state == "" || state == "suspended"
	}
}

// unsuspend clears spec.suspend on the live workload so a suspended workload resumes.
// It is a stateAction used by resume flows: create suspended, observe Suspended, then
// unsuspend and watch the workload run to completion.
func unsuspend(ctx context.Context, obj *unstructured.Unstructured) error {
	live := emptyLike(obj)
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), live); err != nil {
		return err
	}
	if err := unstructured.SetNestedField(live.Object, false, "spec", "suspend"); err != nil {
		return err
	}
	return k8sClient.Update(ctx, live)
}
