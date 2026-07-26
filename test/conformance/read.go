// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"context"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// Read runs the Karta library on one CR and returns the statuses it matches. It is the single read
// path shared by the live recorder and the offline transition test, so what Karta reads of a CR can
// never drift between record and replay. The CR is passed as-is (no sanitize): Karta reads state from
// the fields the definition names, so per-run volatile fields never change the result.
func Read(karta *v1alpha1.Karta, obj resource.KubernetesObject) ([]v1alpha1.ResourceStatus, error) {
	root, err := resource.NewComponentFactoryFromObject(karta, obj).GetRootComponent()
	if err != nil {
		return nil, err
	}
	status, err := root.GetStatus(context.Background())
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, nil
	}
	return status.MatchedStatuses, nil
}
