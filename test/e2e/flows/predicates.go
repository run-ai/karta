// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/test/e2e/recorder"
)

// State predicates: each reads a workload's own fields to recognise one state, never Karta.

// AllOf matches when every check matches, for a state read from more than one condition (a Deployment
// is initializing while Progressing is True and Available is False).
func AllOf(checks ...recorder.StateCheck) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		for _, c := range checks {
			if !c(u) {
				return false
			}
		}
		return len(checks) > 0
	}
}

// CondTrue matches when any of the given condition types is present with status True.
func CondTrue(condTypes ...string) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			m, ok := c.(map[string]any)
			if !ok || m["status"] != "True" {
				continue
			}
			for _, t := range condTypes {
				if m["type"] == t {
					return true
				}
			}
		}
		return false
	}
}

// CondFalse matches when a condition of the given type is present and False.
func CondFalse(condType string) recorder.StateCheck {
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

// CondNotTrue matches when no condition of condType is present with status True (absent or non-True). A
// just-created Deployment is Progressing with no Available condition yet, still initializing.
func CondNotTrue(condType string) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			if m, ok := c.(map[string]any); ok && m["type"] == condType && m["status"] == "True" {
				return false
			}
		}
		return true
	}
}

// CondReason matches when the condition of the given type is True with the given reason. A Deployment is
// Running only when Progressing is True with reason NewReplicaSetAvailable, so status alone is not enough.
func CondReason(condType, reason string) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			if m, ok := c.(map[string]any); ok && m["type"] == condType && m["status"] == "True" && m["reason"] == reason {
				return true
			}
		}
		return false
	}
}

func PhaseEq(want string, path ...string) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, _, _ := unstructured.NestedString(u.Object, path...)
		return got == want
	}
}

func IntAtLeast(n int64, path ...string) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got >= n
	}
}

// IntEq matches when the integer field at the given path is present and exactly n. It gates a scale flow's
// step for a workload whose readiness count is a single field (Grove reports status.availableReplicas).
func IntEq(n int64, path ...string) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got == n
	}
}

// ReplicasReady matches a workload settled at exactly n replicas: status.replicas and status.readyReplicas
// both equal n. It gates a scale flow's step so each replica count is captured only once the controller
// has finished scaling to it, not mid-rollout.
func ReplicasReady(n int64) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		replicas, ok, _ := unstructured.NestedInt64(u.Object, "status", "replicas")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		return ok && replicas == n && ready == n
	}
}

// ReplicasDegraded matches a settled-degraded workload: every desired replica created (updatedReplicas ==
// spec.replicas) but some not ready (0 < readyReplicas < spec.replicas).
func ReplicasDegraded() recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		desired, ok, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
		if !ok {
			desired = 1
		}
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		updated, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
		return desired > 0 && updated == desired && ready > 0 && ready < desired
	}
}

// JobDegraded matches a parallel Job settled degraded: parallelism > 1, some but not all pods ready, and
// at least one pod already succeeded or failed - the Job analog of ReplicasDegraded.
func JobDegraded() recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		par, ok, _ := unstructured.NestedInt64(u.Object, "spec", "parallelism")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "ready")
		succeeded, _, _ := unstructured.NestedInt64(u.Object, "status", "succeeded")
		failedN, _, _ := unstructured.NestedInt64(u.Object, "status", "failed")
		return ok && par > 1 && ready > 0 && ready < par && (succeeded > 0 || failedN > 0)
	}
}

// JobsetRunning matches a JobSet with working pods: at least one replicatedJob has active or ready pods and
// none have failed. Reading either count (not both) keeps the state stable while the controller briefly
// flaps ready to 0 mid-run.
func JobsetRunning() recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		rjs, _, _ := unstructured.NestedSlice(u.Object, "status", "replicatedJobsStatus")
		anyWorking := false
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
			if ready > 0 || active > 0 {
				anyWorking = true
			}
		}
		return anyWorking
	}
}

// JobsetInitializing matches a JobSet in progress with no working pods: status exists, no replicatedJob
// has active or ready pods, and no terminal or suspended condition is set. Covers a just-created JobSet
// (all counts zero) and the window after a job succeeds but before the JobSet-level Completed condition is
// set.
func JobsetInitializing() recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		rjs, _, _ := unstructured.NestedSlice(u.Object, "status", "replicatedJobsStatus")
		if len(rjs) == 0 {
			return false
		}
		if CondTrue("Completed", "Failed", "Suspended")(u) {
			return false
		}
		for _, r := range rjs {
			m, ok := r.(map[string]any)
			if !ok {
				continue
			}
			ready, _, _ := unstructured.NestedInt64(m, "ready")
			active, _, _ := unstructured.NestedInt64(m, "active")
			if ready > 0 || active > 0 {
				return false
			}
		}
		return true
	}
}
