// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MapCRDToKartaEvent maps a CRD event to reconcile requests for every Karta
// whose root component references the same group and kind as that CRD,
// regardless of version.
//
// Matching on group+kind (not the full GVK) ensures that we enqueue Kartas
// when a CRD update removes or stops serving a version they reference. Under
// version-only matching the Karta would be invisible to the mapper the moment
// its referenced version disappears from the served set - the reconciler would
// never run and CRDExists would remain stale. The reconciler (Story 1.3)
// re-checks the exact version and sets CRDExists correctly.
func (r *Reconciler) MapCRDToKartaEvent(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)

	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		logger.Error(fmt.Errorf("unexpected type %T", obj), "expected CustomResourceDefinition")
		return nil
	}

	crdGK := schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind}

	kartas := &kartav1alpha1.KartaList{}
	if err := r.List(ctx, kartas); err != nil {
		logger.Error(err, "Failed to list Kartas for CRD event", "crd", crd.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(kartas.Items))
	for i := range kartas.Items {
		karta := &kartas.Items[i]
		gvk := rootGVK(karta)
		if gvk == nil {
			continue
		}
		if gvk.GroupKind() != crdGK {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: karta.Name},
		})
	}
	return requests
}

// rootGVK extracts the GroupVersionKind of the Karta root component, or nil
// when the Karta has no root component kind defined.
func rootGVK(karta *kartav1alpha1.Karta) *schema.GroupVersionKind {
	kind := karta.Spec.StructureDefinition.RootComponent.Kind
	if kind == nil {
		return nil
	}
	return &schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}
}
