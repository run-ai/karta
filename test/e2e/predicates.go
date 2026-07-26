// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Shared state predicates: each reads a workload's own fields to recognise one stable
// state. The pod case uses phaseEq; the rest serve the per-operator cases in follow-up PRs.

func condTrue(condType string) stateCheck {
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

// condFalse matches when a condition of the given type is present and False.
func condFalse(condType string) stateCheck {
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

func phaseEq(want string, path ...string) stateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, _, _ := unstructured.NestedString(u.Object, path...)
		return got == want
	}
}

// phaseAny matches when the string at the path equals any of wants (some operators map
// one state from several phase strings).
func phaseAny(wants []string, path ...string) stateCheck {
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

func intAtLeast(min int64, path ...string) stateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got >= min
	}
}

// replicasReady matches a workload settled at exactly n replicas: status.replicas and
// status.readyReplicas both equal n. It gates a scale flow's step so each replica count is captured
// only once the controller has finished scaling to it, not mid-rollout.
func replicasReady(n int64) stateCheck {
	return func(u *unstructured.Unstructured) bool {
		replicas, ok, _ := unstructured.NestedInt64(u.Object, "status", "replicas")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		return ok && replicas == n && ready == n
	}
}

// fullyAvailable matches a replicated workload with every replica ready (replicas > 0 and
// readyReplicas == replicas), at any count. It is the running state for a Deployment or StatefulSet:
// the rollout is complete, which is when Karta reads it as Running. During a scale it goes false
// mid-rollout, so a scale flow only classifies Running at the settled counts.
func fullyAvailable() stateCheck {
	return func(u *unstructured.Unstructured) bool {
		replicas, ok, _ := unstructured.NestedInt64(u.Object, "status", "replicas")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		return ok && replicas > 0 && ready == replicas
	}
}

// replicasDegraded matches a replicated workload that has settled degraded: the
// controller has created every replica (updatedReplicas == replicas) but some are
// not ready (0 < readyReplicas < replicas). Requiring updatedReplicas == replicas
// excludes the transient mid-rollout where a StatefulSet has only brought up its
// first ordinals, so the assertion never races the workload into a false Degraded.
func replicasDegraded() stateCheck {
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
func intBelow(max int64, path ...string) stateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got < max
	}
}

func boolTrue(path ...string) stateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedBool(u.Object, path...)
		return err == nil && found && got
	}
}

// absent matches when the field at the given path is not present, for example a
// CronJob that has not scheduled yet (status.lastScheduleTime missing).
func absent(path ...string) stateCheck {
	return func(u *unstructured.Unstructured) bool {
		_, found, _ := unstructured.NestedFieldNoCopy(u.Object, path...)
		return !found
	}
}

// cronjobFired matches a CronJob that has scheduled at least once
// (status.lastScheduleTime set) and whose spawned Job has finished
// (status.active empty), so the recorded snapshot carries no timestamped active-Job
// reference. Karta reads a fired, enabled CronJob as Running.
func cronjobFired() stateCheck {
	return func(u *unstructured.Unstructured) bool {
		_, scheduled, _ := unstructured.NestedFieldNoCopy(u.Object, "status", "lastScheduleTime")
		active, _, _ := unstructured.NestedSlice(u.Object, "status", "active")
		return scheduled && len(active) == 0
	}
}

// condsFalse matches when every listed status condition is present and False, for
// example the KServe failed mapping (PredictorReady, PredictorConfigurationReady, and
// RoutesReady all False).
func condsFalse(types ...string) stateCheck {
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
// ready and active with no failures (the definition's running expression).
func jobsetRunning() stateCheck {
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
// replicatedJob has active > 0 and ready == 0, none failed. It mirrors the definition's
// initializing expression and is the ready==0 counterpart of jobsetRunning, so a
// readiness probe on the job pods yields Initializing before Running.
func jobsetInitializing() stateCheck {
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
func jobDegraded() stateCheck {
	return func(u *unstructured.Unstructured) bool {
		par, ok, _ := unstructured.NestedInt64(u.Object, "spec", "parallelism")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "ready")
		succeeded, _, _ := unstructured.NestedInt64(u.Object, "status", "succeeded")
		failedN, _, _ := unstructured.NestedInt64(u.Object, "status", "failed")
		return ok && par > 1 && ready > 0 && ready < par && (succeeded > 0 || failedN > 0)
	}
}

// raySuspended matches a RayCluster suspended at creation: spec.suspend is true and
// status.state is either "suspended" or not yet set (the definition's suspended expr,
// which also matches when the operator has not written a state yet).
func raySuspended() stateCheck {
	return func(u *unstructured.Unstructured) bool {
		if suspend, _, _ := unstructured.NestedBool(u.Object, "spec", "suspend"); !suspend {
			return false
		}
		state, found, _ := unstructured.NestedString(u.Object, "status", "state")
		return !found || state == "" || state == "suspended"
	}
}
