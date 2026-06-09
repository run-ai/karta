// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import "strings"

const (
	// LabelGVK is the label key stamped by the OSS Karta operator on every
	// Karta whose root component has a GVK defined.
	//
	// The value is the root component's GroupVersionKind encoded as:
	//
	//   "<group>__<version>__<kind>"
	//
	// e.g. "ray.io__v1__RayCluster"
	//
	// Consumers can locate a Karta for an exact GVK with:
	//
	//   client.MatchingLabels{v1alpha1.LabelGVK: v1alpha1.FormatGVKLabel(group, version, kind)}
	//
	// The label is kept in sync by the operator on every reconcile.
	// Consumers must not write this label; the operator is the sole owner.
	LabelGVK = "karta/gvk"
)

// FormatGVKLabel encodes a GroupVersionKind as the value for LabelGVK.
func FormatGVKLabel(group, version, kind string) string {
	return strings.Join([]string{group, version, kind}, "__")
}
