// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Shared state predicates: each reads a workload's own fields to recognise one stable
// state. The pod case uses PhaseEq; the rest serve the per-operator cases in follow-up PRs.

// AllOf matches when every check matches, for a state a definition reads from more than one
// condition (a Deployment is initializing while Progressing is True and Available is False).
func AllOf(checks ...StateCheck) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		for _, c := range checks {
			if !c(u) {
				return false
			}
		}
		return len(checks) > 0
	}
}

func CondTrue(condType string) StateCheck {
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

// CondFalse matches when a condition of the given type is present and False.
func CondFalse(condType string) StateCheck {
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

func PhaseEq(want string, path ...string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, _, _ := unstructured.NestedString(u.Object, path...)
		return got == want
	}
}

// PhaseAny matches when the string at the path equals any of wants (some operators map
// one state from several phase strings).
func PhaseAny(wants []string, path ...string) StateCheck {
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

func IntAtLeast(min int64, path ...string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got >= min
	}
}

// ReplicasReady matches a workload settled at exactly n replicas: status.replicas and
// status.readyReplicas both equal n. It gates a scale flow's step so each replica count is captured
// only once the controller has finished scaling to it, not mid-rollout.
func ReplicasReady(n int64) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		replicas, ok, _ := unstructured.NestedInt64(u.Object, "status", "replicas")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		return ok && replicas == n && ready == n
	}
}

// IntEq matches when the integer field at the given path is present and exactly n. It gates a scale
// flow's step for a workload whose readiness count is a single field (a Grove PodCliqueSet reports
// status.availableReplicas), where ReplicasReady's replicas+readyReplicas pair does not apply.
func IntEq(n int64, path ...string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got == n
	}
}

// FullyAvailable matches when every desired replica is created and ready (readyReplicas == updatedReplicas
// == spec.replicas), Karta's Running for a StatefulSet. It compares to spec.replicas, not the lagging
// status.replicas, so a gradually-scaled StatefulSet never reads Running mid-ramp.
func FullyAvailable() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		desired, ok, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
		if !ok {
			desired = 1 // Karta defaults `.spec.replicas // 1`
		}
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		updated, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
		return desired > 0 && ready == desired && updated == desired
	}
}

// ReplicasDegraded matches a settled-degraded workload: every desired replica created (updatedReplicas ==
// spec.replicas) but some not ready (0 < readyReplicas < spec.replicas). Compares to spec.replicas.
func ReplicasDegraded() StateCheck {
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

// ReplicasInitializing matches a StatefulSet still coming up: spec.replicas > 0 and either nothing ready
// (readyReplicas == 0) or not all created (updatedReplicas != spec.replicas). Mirrors Karta's initializing.
func ReplicasInitializing() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		desired, ok, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
		if !ok {
			desired = 1
		}
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		updated, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
		return desired > 0 && (ready == 0 || updated != desired)
	}
}

// ReplicasSettled matches when every current replica is ready and updated (readyReplicas == updatedReplicas
// == status.replicas), Karta's Running for a LeaderWorkerSet, which compares to status.replicas not spec.
func ReplicasSettled() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		replicas, ok, _ := unstructured.NestedInt64(u.Object, "status", "replicas")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		updated, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
		return ok && replicas > 0 && ready == replicas && updated == replicas
	}
}

// CondReason matches when the condition of the given type is True with the given reason. A Deployment is
// Running only when Progressing is True with reason NewReplicaSetAvailable, so status alone is not enough.
func CondReason(condType, reason string) StateCheck {
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

// AllReplicasAvailable matches when every desired replica is available (status.availableReplicas >=
// spec.replicas), Karta's Running for a Grove PodCliqueSet. Includes the vacuous spec.replicas == 0 case.
func AllReplicasAvailable() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
		avail, _, _ := unstructured.NestedInt64(u.Object, "status", "availableReplicas")
		return avail >= desired
	}
}

// ReplicasComingUp is the initializing counterpart of AllReplicasAvailable: spec.replicas > 0 and not
// every desired replica is available yet (status.availableReplicas < spec.replicas).
func ReplicasComingUp() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
		avail, _, _ := unstructured.NestedInt64(u.Object, "status", "availableReplicas")
		return desired > 0 && avail < desired
	}
}

func BoolTrue(path ...string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedBool(u.Object, path...)
		return err == nil && found && got
	}
}

// Absent matches when the field at the given path is not present, for example a
// CronJob that has not scheduled yet (status.lastScheduleTime missing).
func Absent(path ...string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		_, found, _ := unstructured.NestedFieldNoCopy(u.Object, path...)
		return !found
	}
}

// CronjobFired matches a CronJob that has scheduled at least once
// (status.lastScheduleTime set) and whose spawned Job has finished
// (status.active empty), so the recorded snapshot carries no timestamped active-Job
// reference. Karta reads a fired, enabled CronJob as Running.
func CronjobFired() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		// Karta reads a CronJob as running once it has fired and is not suspended, whether or not a Job
		// is currently active - so match on lastScheduleTime alone (suspended wins via the registry).
		_, scheduled, _ := unstructured.NestedFieldNoCopy(u.Object, "status", "lastScheduleTime")
		return scheduled
	}
}

// CondsFalse matches when every listed status condition is present and False, for
// example the KServe failed mapping (PredictorReady, PredictorConfigurationReady, and
// RoutesReady all False).
func CondsFalse(types ...string) StateCheck {
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

// JobsetRunning matches a JobSet with a running replicated job: some replicatedJob is
// ready and active with no failures (the definition's running expression).
func JobsetRunning() StateCheck {
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

// JobsetInitializing matches a JobSet whose pods are active but not yet ready: some
// replicatedJob has active > 0 and ready == 0, none failed. It mirrors the definition's
// initializing expression and is the ready==0 counterpart of JobsetRunning, so a
// readiness probe on the job pods yields Initializing before Running.
func JobsetInitializing() StateCheck {
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

// JobDegraded matches a parallel Job settled degraded: parallelism > 1, some but not
// all pods ready, and at least one pod already succeeded or failed - the Job analog of
// ReplicasDegraded.
func JobDegraded() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		par, ok, _ := unstructured.NestedInt64(u.Object, "spec", "parallelism")
		ready, _, _ := unstructured.NestedInt64(u.Object, "status", "ready")
		succeeded, _, _ := unstructured.NestedInt64(u.Object, "status", "succeeded")
		failedN, _, _ := unstructured.NestedInt64(u.Object, "status", "failed")
		return ok && par > 1 && ready > 0 && ready < par && (succeeded > 0 || failedN > 0)
	}
}

// RaySuspended matches a RayCluster suspended at creation: spec.suspend is true and
// status.state is either "suspended" or not yet set (the definition's suspended expr,
// which also matches when the operator has not written a state yet).
func RaySuspended() StateCheck {
	return func(u *unstructured.Unstructured) bool {
		if suspend, _, _ := unstructured.NestedBool(u.Object, "spec", "suspend"); !suspend {
			return false
		}
		state, found, _ := unstructured.NestedString(u.Object, "status", "state")
		return !found || state == "" || state == "suspended"
	}
}
