// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package definitions

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// GVR is the cluster-scoped resource the CLI reads Karta definitions from.
var GVR = schema.GroupVersionResource{Group: "run.ai", Version: "v1alpha1", Resource: "kartas"}

// listWithClient reads every Karta in the cluster through a dynamic client.
// Kartas are cluster-scoped, so the request carries no namespace and no selector.
func listWithClient(ctx context.Context, dyn dynamic.Interface) ([]*v1alpha1.Karta, error) {
	list, err := dyn.Resource(GVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Karta definitions: %w", err)
	}

	kartas := make([]*v1alpha1.Karta, 0, len(list.Items))
	for i := range list.Items {
		var karta v1alpha1.Karta
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[i].Object, &karta); err != nil {
			return nil, fmt.Errorf("convert Karta definition %q: %w", list.Items[i].GetName(), err)
		}
		kartas = append(kartas, &karta)
	}
	return kartas, nil
}

// FetchCluster reads the Karta definitions installed in the cluster that rcg
// points at. An empty cluster yields an empty result and no error.
func FetchCluster(ctx context.Context, rcg genericclioptions.RESTClientGetter) ([]*v1alpha1.Karta, error) {
	cfg, err := rcg.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes config: %w", err)
	}

	// Silence deprecation warnings for this config only. The process-global
	// rest.SetDefaultWarningHandler would leak the change to every other client,
	// and a warning printed by the default handler would land in the middle of
	// machine-readable output.
	cfg.WarningHandler = rest.NoWarnings{}

	// Timeout, QPS and Burst are deliberately left as the config getter produced
	// them: genericclioptions owns --request-timeout and a single list needs no
	// rate-limit tuning. Cancellation comes from ctx.
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes dynamic client: %w", err)
	}
	return listWithClient(ctx, dyn)
}
