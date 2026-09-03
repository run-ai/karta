// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// maxOwnerDepth bounds the owner-reference walk. Real controller chains (root
// -> intermediate -> Pod) are at most two or three hops; this guards against a
// cycle in adversarial or malformed data.
const maxOwnerDepth = 6

// podsGVR is the fixed core/v1 Pod resource, which discovery does not need to map.
var podsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

// PodAttributor attributes live Pods to a workload root by walking each
// candidate pod's owner-reference chain, fetching intermediate controller
// objects (ReplicaSet, Job, and so on) as needed. It caches fetched objects
// across calls, since sibling pods usually share an intermediate owner.
//
// A Karta PodSelector says which component type a pod plays, never which
// workload it belongs to, so ownership is what scopes a namespace of pods down
// to one workload.
type PodAttributor struct {
	dyn    dynamic.Interface
	mapper meta.RESTMapper
	cache  map[ownerKey]*unstructured.Unstructured
}

type ownerKey struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

func NewPodAttributor(dyn dynamic.Interface, mapper meta.RESTMapper) *PodAttributor {
	return &PodAttributor{dyn: dyn, mapper: mapper, cache: map[ownerKey]*unstructured.Unstructured{}}
}

// Filter returns the pods whose owner-reference chain reaches rootUID. pods
// may span namespaces, as in a cluster-wide search.
func (a *PodAttributor) Filter(ctx context.Context, pods []corev1.Pod, rootUID types.UID) []corev1.Pod {
	var matched []corev1.Pod
	for i := range pods {
		if a.belongsTo(ctx, pods[i].OwnerReferences, pods[i].Namespace, rootUID, 0) {
			matched = append(matched, pods[i])
		}
	}
	return matched
}

// belongsTo reports whether any owner in refs is, or transitively leads to,
// rootUID.
func (a *PodAttributor) belongsTo(
	ctx context.Context, refs []metav1.OwnerReference, namespace string, rootUID types.UID, depth int,
) bool {
	if depth >= maxOwnerDepth {
		return false
	}
	for _, ref := range refs {
		if ref.UID == rootUID {
			return true
		}
	}
	controller := controllerRef(refs)
	if controller == nil {
		return false
	}
	owner, err := a.get(ctx, *controller, namespace)
	if err != nil || owner == nil {
		return false
	}
	return a.belongsTo(ctx, owner.GetOwnerReferences(), namespace, rootUID, depth+1)
}

// controllerRef returns the single owner reference with Controller set, the
// only one an owner chain can be walked through unambiguously.
func controllerRef(refs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			return &refs[i]
		}
	}
	return nil
}

func (a *PodAttributor) get(
	ctx context.Context, ref metav1.OwnerReference, namespace string,
) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("parse owner apiVersion %q: %w", ref.APIVersion, err)
	}
	gvk := gv.WithKind(ref.Kind)
	key := ownerKey{gvk: gvk, namespace: namespace, name: ref.Name}
	if cached, ok := a.cache[key]; ok {
		return cached, nil
	}

	mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", gvk.Kind, err)
	}
	obj, err := a.dyn.Resource(mapping.Resource).Namespace(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	// Cache the miss too, so a broken chain is not re-fetched once per pod.
	a.cache[key] = obj
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// ListPods reads every pod in namespace once, decoded to the typed shape the
// attributor and the view both need. An empty namespace lists cluster-wide.
func ListPods(ctx context.Context, dyn dynamic.Interface, namespace string) ([]corev1.Pod, error) {
	list, err := dyn.Resource(podsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0, len(list.Items))
	for i := range list.Items {
		var pod corev1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[i].Object, &pod); err != nil {
			return nil, fmt.Errorf("decode pod %s: %w", list.Items[i].GetName(), err)
		}
		pods = append(pods, pod)
	}
	return pods, nil
}
