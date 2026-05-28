// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package internal

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
// The lookup uses karta.run.ai/group + karta.run.ai/kind label selectors
// (stamped by stepEnsureLabels during every reconcile) so that the mapper
// issues a filtered API call instead of fetching all Kartas.
//
// Matching on group+kind (not the full GVK) means that when a CRD update
// removes or stops serving a version, all Kartas referencing any version of
// that group/kind are enqueued. The reconciler's stepCheckCRDExists then
// re-evaluates the exact version and sets CRDExists accordingly.
//
// Note: a Karta that was just created but not yet reconciled will not have
// the index labels and will therefore not be found here. This is acceptable
// because its own create event will trigger a direct reconcile, which stamps
// the labels for subsequent CRD events.
func (r *Reconciler) MapCRDToKartaEvent(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)

	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		logger.Error(fmt.Errorf("unexpected type %T", obj), "expected CustomResourceDefinition")
		return nil
	}

	kartas := &kartav1alpha1.KartaList{}
	if err := r.List(ctx, kartas, client.MatchingLabels{
		kartav1alpha1.LabelRootGroup: crd.Spec.Group,
		kartav1alpha1.LabelRootKind:  crd.Spec.Names.Kind,
	}); err != nil {
		logger.Error(err, "Failed to list Kartas for CRD event", "crd", crd.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(kartas.Items))
	for i := range kartas.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: kartas.Items[i].Name},
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
