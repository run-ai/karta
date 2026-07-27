// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// k8sClient is the cluster client the actions patch through. The e2e harness sets it once from
// BeforeSuite via SetClient, so the case definitions can stay plain data.
var k8sClient client.Client

// SetClient wires the cluster client the actions use. Called by the e2e suite before any flow runs.
func SetClient(c client.Client) { k8sClient = c }

// EmptyLike returns an empty unstructured object carrying only src's GroupVersionKind, a target for
// a merge patch that must not race the controller by sending back a stale spec or status.
func EmptyLike(src *unstructured.Unstructured) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(src.GroupVersionKind())
	return u
}

// Unsuspend clears spec.suspend so a suspended workload resumes. A merge patch, not a
// read-modify-write Update, so it does not race the controller reconciling a just-created
// workload, which would make an Update conflict on the resourceVersion.
func Unsuspend(ctx context.Context, obj *unstructured.Unstructured) error {
	target := EmptyLike(obj)
	target.SetName(obj.GetName())
	target.SetNamespace(obj.GetNamespace())
	return k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"suspend":false}}`)))
}

// ScaleParallelism patches spec.parallelism, the Job analog of ScaleReplicas, to drive a batch Job's
// pod count up or down. A merge patch, so it does not race the controller on the resourceVersion.
func ScaleParallelism(n int) StateAction {
	return func(ctx context.Context, obj *unstructured.Unstructured) error {
		target := EmptyLike(obj)
		target.SetName(obj.GetName())
		target.SetNamespace(obj.GetNamespace())
		patch := []byte(fmt.Sprintf(`{"spec":{"parallelism":%d}}`, n))
		return k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, patch))
	}
}

// UnsuspendRunPolicy clears spec.runPolicy.suspend so a suspended Kubeflow job (PyTorchJob, MPIJob)
// resumes. Kubeflow puts the suspend flag under runPolicy, unlike the top-level spec.suspend that
// Unsuspend patches for batch-job, jobset, and raycluster.
func UnsuspendRunPolicy(ctx context.Context, obj *unstructured.Unstructured) error {
	target := EmptyLike(obj)
	target.SetName(obj.GetName())
	target.SetNamespace(obj.GetNamespace())
	return k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"runPolicy":{"suspend":false}}}`)))
}

// ScaleReplicas patches spec.replicas, the merge-patch analog of kubectl scale, to drive a scale up
// or down on any workload with a scalar spec.replicas. A merge patch, not a read-modify-write Update,
// so it does not race the controller on the resourceVersion.
func ScaleReplicas(n int) StateAction {
	return func(ctx context.Context, obj *unstructured.Unstructured) error {
		target := EmptyLike(obj)
		target.SetName(obj.GetName())
		target.SetNamespace(obj.GetNamespace())
		patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, n))
		return k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, patch))
	}
}
