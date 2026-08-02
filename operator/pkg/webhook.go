// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-run-ai-v1alpha1-karta,mutating=true,failurePolicy=ignore,sideEffects=None,groups=run.ai,resources=kartas,verbs=create,versions=v1alpha1,name=mkarta.run.ai,admissionReviewVersions=v1

// KartaLabeler stamps the GVK index labels on a Karta at admission (issue #99).
type KartaLabeler struct{}

var _ admission.Defaulter[*kartav1alpha1.Karta] = (*KartaLabeler)(nil)

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kartav1alpha1.Karta{}).
		WithDefaulter(&KartaLabeler{}).
		WithValidator(&KartaValidator{reader: mgr.GetAPIReader()}).
		Complete()
}

func (w *KartaLabeler) Default(_ context.Context, karta *kartav1alpha1.Karta) error {
	desired := desiredRootLabels(karta)
	if desired == nil {
		return nil
	}
	if karta.Labels == nil {
		karta.Labels = make(map[string]string, len(desired))
	}
	for k, v := range desired {
		karta.Labels[k] = v
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-run-ai-v1alpha1-karta,mutating=false,failurePolicy=ignore,sideEffects=None,groups=run.ai,resources=kartas,verbs=create;update,versions=v1alpha1,name=vkarta.run.ai,admissionReviewVersions=v1

// KartaValidator validates a Karta at admission: spec validity and one Karta per
// root GVK (issue #198).
type KartaValidator struct {
	// reader reads from the apiserver, not the cache, so the uniqueness check is current.
	reader client.Reader
}

var _ admission.Validator[*kartav1alpha1.Karta] = (*KartaValidator)(nil)

func (v *KartaValidator) ValidateCreate(ctx context.Context, karta *kartav1alpha1.Karta) (admission.Warnings, error) {
	return nil, v.validate(ctx, karta)
}

func (v *KartaValidator) ValidateUpdate(ctx context.Context, _, karta *kartav1alpha1.Karta) (admission.Warnings, error) {
	return nil, v.validate(ctx, karta)
}

func (v *KartaValidator) ValidateDelete(_ context.Context, _ *kartav1alpha1.Karta) (admission.Warnings, error) {
	return nil, nil
}

func (v *KartaValidator) validate(ctx context.Context, karta *kartav1alpha1.Karta) error {
	if err := kartav1alpha1.NewKartaValidator(karta).Validate(); err != nil {
		return err
	}
	return v.validateUniqueGVK(ctx, karta)
}

// validateUniqueGVK enforces one Karta per root GVK. Best-effort: admission is not
// transactional, so two created at once can both pass; the reconciler is the backstop.
func (v *KartaValidator) validateUniqueGVK(ctx context.Context, karta *kartav1alpha1.Karta) error {
	gvk := rootGVK(karta)
	if gvk == nil {
		return nil
	}

	kartas := &kartav1alpha1.KartaList{}
	if err := v.reader.List(ctx, kartas); err != nil {
		return fmt.Errorf("list Kartas to enforce GVK uniqueness: %w", err)
	}

	for i := range kartas.Items {
		other := &kartas.Items[i]
		if other.Name == karta.Name {
			continue
		}
		if og := rootGVK(other); og != nil && *og == *gvk {
			return fmt.Errorf("a Karta already exists for GVK %s/%s %s: %q",
				gvk.Group, gvk.Version, gvk.Kind, other.Name)
		}
	}
	return nil
}
