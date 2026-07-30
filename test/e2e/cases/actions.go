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

var k8sClient client.Client

func SetClient(c client.Client) { k8sClient = c }

// GVKOnly is a GVK-only object, so a merge-patch never sends back a stale spec or status.
func GVKOnly(src *unstructured.Unstructured) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(src.GroupVersionKind())
	return u
}

func Unsuspend(ctx context.Context, obj *unstructured.Unstructured) error {
	target := GVKOnly(obj)
	target.SetName(obj.GetName())
	target.SetNamespace(obj.GetNamespace())
	if err := k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"suspend":false}}`))); err != nil {
		return fmt.Errorf("unsuspend %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

func ScaleParallelism(n int) StateAction {
	return func(ctx context.Context, obj *unstructured.Unstructured) error {
		target := GVKOnly(obj)
		target.SetName(obj.GetName())
		target.SetNamespace(obj.GetNamespace())
		patch := []byte(fmt.Sprintf(`{"spec":{"parallelism":%d}}`, n))
		if err := k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, patch)); err != nil {
			return fmt.Errorf("scale parallelism %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
		return nil
	}
}
