// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// PodMatcher decouples tree construction from the rule that decides which
// pods belong to which component. The builder walks the component hierarchy
// top-down and asks the matcher whether a given pod belongs to (or beneath)
// a given component; the matcher's strategy is opaque to the builder.
type PodMatcher interface {
	// Matches reports whether pod belongs to component or any of its
	// descendants. Returning true on a non-leaf component does not yet pin
	// the pod to a specific child; the builder narrows the candidate set as
	// it descends.
	Matches(ctx context.Context, pod *corev1.Pod, component *v1alpha1.ComponentDefinition) (bool, error)
}

// JQMatcher matches pods to components using the Karta definition's
// ComponentTypeSelector — a JQ path on the pod plus an optional expected
// value. This mirrors how the existing pkg/resource layer interprets pod
// selectors and is the right default for any Karta definition.
//
// Components with no PodSelector or no ComponentTypeSelector are treated as
// matching every pod, which matches the resource layer's behavior for
// container or grouping components.
type JQMatcher struct{}

// Matches implements PodMatcher.
func (JQMatcher) Matches(ctx context.Context, pod *corev1.Pod, component *v1alpha1.ComponentDefinition) (bool, error) {
	if component == nil || component.PodSelector == nil || component.PodSelector.ComponentTypeSelector == nil {
		return true, nil
	}
	return resource.NewPodQuerier(pod).MatchesComponentType(ctx, component.PodSelector.ComponentTypeSelector)
}
