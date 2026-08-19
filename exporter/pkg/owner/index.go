// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package owner

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// maxWalkDepth caps the owner-reference walk so a reference cycle cannot
// loop forever.
const maxWalkDepth = 8

// Ref identifies an object reached through the owner walk.
type Ref struct {
	UID  types.UID
	GVK  schema.GroupVersionKind
	Name string
}

// WalkOutcome describes how a RootFor walk ended.
type WalkOutcome string

const (
	// OutcomeFound means the walk reached a registered root kind.
	OutcomeFound WalkOutcome = "found"
	// OutcomeNoController means an object in the chain has no controller owner.
	OutcomeNoController WalkOutcome = "no_controller"
	// OutcomeMissing means a middle owner is not in the index yet; the walk
	// can be retried once that owner is observed.
	OutcomeMissing WalkOutcome = "missing"
	// OutcomeDepthExceeded means the chain is longer than maxWalkDepth.
	OutcomeDepthExceeded WalkOutcome = "depth_exceeded"
)

// WalkResult is the outcome of a RootFor walk. Missing holds the UID of the
// first absent owner when Outcome is OutcomeMissing.
type WalkResult struct {
	Outcome WalkOutcome
	Root    Ref
	Missing types.UID
}

type node struct {
	owner *metav1.OwnerReference
}

// Index tracks controller owner edges for the middle objects between pods
// and workload roots (Job, StatefulSet, ...), fed by metadata informers.
// It also parks pods whose walk dead-ended on a missing owner, so they can
// be re-attributed when that owner is observed.
type Index struct {
	mu      sync.RWMutex
	nodes   map[types.UID]node
	pending map[types.UID]map[string]struct{}
}

func New() *Index {
	return &Index{
		nodes:   make(map[types.UID]node),
		pending: make(map[types.UID]map[string]struct{}),
	}
}

// UpsertObject records the controller owner edge of a middle object and
// returns the namespace/name keys of pods that were waiting for it.
func (i *Index) UpsertObject(uid types.UID, ownerRefs []metav1.OwnerReference) []string {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.nodes[uid] = node{owner: controllerRef(ownerRefs)}

	waiting := i.pending[uid]
	delete(i.pending, uid)
	drained := make([]string, 0, len(waiting))
	for podKey := range waiting {
		drained = append(drained, podKey)
	}
	return drained
}

func (i *Index) DeleteObject(uid types.UID) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.nodes, uid)
	delete(i.pending, uid)
}

// MarkPending parks a pod (by namespace/name key) on the missing owner that
// blocked its walk.
func (i *Index) MarkPending(missingOwner types.UID, podKey string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	waiting, ok := i.pending[missingOwner]
	if !ok {
		waiting = make(map[string]struct{})
		i.pending[missingOwner] = waiting
	}
	waiting[podKey] = struct{}{}
}

// ForgetPending removes a pod from every pending bucket, for pod deletion.
func (i *Index) ForgetPending(podKey string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for missingOwner, waiting := range i.pending {
		delete(waiting, podKey)
		if len(waiting) == 0 {
			delete(i.pending, missingOwner)
		}
	}
}

// PendingCount returns the number of parked pods.
func (i *Index) PendingCount() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	count := 0
	for _, waiting := range i.pending {
		count += len(waiting)
	}
	return count
}

// RootFor walks controller owner references until it reaches a kind for
// which isRoot returns true. Roots are matched by group and kind only:
// owner references may carry any served version.
func (i *Index) RootFor(ownerRefs []metav1.OwnerReference, isRoot func(schema.GroupKind) bool) WalkResult {
	i.mu.RLock()
	defer i.mu.RUnlock()

	current := controllerRef(ownerRefs)
	for depth := 0; depth < maxWalkDepth; depth++ {
		if current == nil {
			return WalkResult{Outcome: OutcomeNoController}
		}

		gvk := refGVK(current)
		if isRoot(gvk.GroupKind()) {
			return WalkResult{
				Outcome: OutcomeFound,
				Root:    Ref{UID: current.UID, GVK: gvk, Name: current.Name},
			}
		}

		middle, ok := i.nodes[current.UID]
		if !ok {
			return WalkResult{Outcome: OutcomeMissing, Missing: current.UID}
		}
		current = middle.owner
	}

	return WalkResult{Outcome: OutcomeDepthExceeded}
}

func controllerRef(ownerRefs []metav1.OwnerReference) *metav1.OwnerReference {
	for index := range ownerRefs {
		ref := ownerRefs[index]
		if ref.Controller != nil && *ref.Controller {
			return &ref
		}
	}
	return nil
}

func refGVK(ref *metav1.OwnerReference) schema.GroupVersionKind {
	groupVersion, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return schema.GroupVersionKind{Kind: ref.Kind}
	}
	return groupVersion.WithKind(ref.Kind)
}
