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
// whose root component references the GVK served by that CRD.
//
// CRDs can serve multiple versions, so a single CRD event can fan out to
// several Kartas. We list all Kartas and select the ones whose root kind
// matches the CRD group+kind and one of the served versions.
func (r *Reconciler) MapCRDToKartaEvent(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)

	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		logger.Error(fmt.Errorf("unexpected type %T", obj), "expected CustomResourceDefinition")
		return nil
	}

	servedGVKs := servedGVKsFromCRD(crd)
	if len(servedGVKs) == 0 {
		return nil
	}

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
		if _, matches := servedGVKs[*gvk]; !matches {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: karta.Name},
		})
	}
	return requests
}

// servedGVKsFromCRD returns the set of GroupVersionKinds served by the given
// CRD. Non-served versions are skipped.
func servedGVKsFromCRD(crd *apiextensionsv1.CustomResourceDefinition) map[schema.GroupVersionKind]struct{} {
	result := make(map[schema.GroupVersionKind]struct{}, len(crd.Spec.Versions))
	for _, v := range crd.Spec.Versions {
		if !v.Served {
			continue
		}
		result[schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: v.Name,
			Kind:    crd.Spec.Names.Kind,
		}] = struct{}{}
	}
	return result
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
