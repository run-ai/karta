// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/test/e2e/recorder"
)

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

func PhaseEq(want string, path ...string) recorder.StateCheck {
	return func(u *unstructured.Unstructured) bool {
		got, _, _ := unstructured.NestedString(u.Object, path...)
		return got == want
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
