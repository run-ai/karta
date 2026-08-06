// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// significantFields drops the fields the apiserver bumps on every event (resourceVersion, managedFields) so
// keep dedups on real changes.
func significantFields(cr *unstructured.Unstructured) map[string]any {
	stripped := cr.DeepCopy().Object
	unstructured.RemoveNestedField(stripped, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(stripped, "metadata", "managedFields")
	return stripped
}

// isStatusSettled reports whether the controller has caught up (observedGeneration >= generation); workloads
// without those fields count as settled.
func isStatusSettled(cr *unstructured.Unstructured) bool {
	gen, hasGen, _ := unstructured.NestedInt64(cr.Object, "metadata", "generation")
	observed, hasObserved, _ := unstructured.NestedInt64(cr.Object, "status", "observedGeneration")
	if !hasGen || !hasObserved {
		return true
	}
	return observed >= gen
}

// blankWithGVK returns a fresh object carrying only src's GVK, so a merge-patch or a Get never sends back a
// stale spec or status.
func blankWithGVK(src *unstructured.Unstructured) *unstructured.Unstructured {
	blank := &unstructured.Unstructured{}
	blank.SetGroupVersionKind(src.GroupVersionKind())
	return blank
}

// dumpStatus renders a CR's status block for the failure messages; safe on a nil object.
func dumpStatus(cr *unstructured.Unstructured) string {
	if cr == nil {
		return "(no object observed)"
	}
	status, _, _ := unstructured.NestedMap(cr.Object, "status")
	b, err := json.MarshalIndent(status, "  ", "  ")
	if err != nil {
		return fmt.Sprintf("(status marshal error: %v)", err)
	}
	return "  " + string(b)
}
