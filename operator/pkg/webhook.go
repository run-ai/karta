// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-run-ai-v1alpha1-karta,mutating=true,failurePolicy=fail,sideEffects=None,groups=run.ai,resources=kartas,verbs=create,versions=v1alpha1,name=mkarta.run.ai,admissionReviewVersions=v1

type KartaLabeler struct{}

var _ admission.Defaulter[*kartav1alpha1.Karta] = (*KartaLabeler)(nil)

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kartav1alpha1.Karta{}).
		WithDefaulter(&KartaLabeler{}).
		WithValidator(&KartaValidator{}).
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

// +kubebuilder:webhook:path=/validate-run-ai-v1alpha1-karta,mutating=false,failurePolicy=fail,sideEffects=None,groups=run.ai,resources=kartas,verbs=create;update,versions=v1alpha1,name=vkarta.run.ai,admissionReviewVersions=v1

type KartaValidator struct{}

var _ admission.Validator[*kartav1alpha1.Karta] = (*KartaValidator)(nil)

func (v *KartaValidator) ValidateCreate(_ context.Context, karta *kartav1alpha1.Karta) (admission.Warnings, error) {
	return nil, kartav1alpha1.NewKartaValidator(karta).Validate()
}

func (v *KartaValidator) ValidateUpdate(_ context.Context, _, karta *kartav1alpha1.Karta) (admission.Warnings, error) {
	return nil, kartav1alpha1.NewKartaValidator(karta).Validate()
}

func (v *KartaValidator) ValidateDelete(_ context.Context, _ *kartav1alpha1.Karta) (admission.Warnings, error) {
	return nil, nil
}
