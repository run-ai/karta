// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

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
// The single karta/gvk label encodes group+version+kind, so it cannot be used
// for a group+kind-only label-selector query. Instead we list all Kartas and
// filter in Go using rootGVK(). This also benefits the bootstrapping case: a
// freshly created Karta whose first reconcile has not yet stamped the label is
// still found here (via spec), closing the window where a concurrent CRD event
// would be missed.
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
		gvk := rootGVK(&kartas.Items[i])
		if gvk == nil || gvk.GroupKind() != crdGK {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: kartas.Items[i].Name},
		})
	}
	return requests
}

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
