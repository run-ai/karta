// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
)

// TrimCRDFields trims each CRD down to only the fields the reconciler
// reads, so that unrelated fields are never retained in the cache.
func TrimCRDFields(i any) (any, error) {
	crd, ok := i.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return i, nil
	}
	versions := make([]apiextensionsv1.CustomResourceDefinitionVersion, len(crd.Spec.Versions))
	for j, v := range crd.Spec.Versions {
		versions[j] = apiextensionsv1.CustomResourceDefinitionVersion{
			Name:   v.Name,
			Served: v.Served,
		}
	}
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:            crd.Name,
			ResourceVersion: crd.ResourceVersion,
			Generation:      crd.Generation,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: crd.Spec.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: crd.Spec.Names.Kind,
			},
			Versions: versions,
		},
	}, nil
}

var _ toolscache.TransformFunc = TrimCRDFields // compile-time signature check
