// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("StatefulSet (built-in)", Ordered, Label("statefulset", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/apps-statefulset-v1.yaml", "apps-statefulset-v1")
		rec = recorder.New("statefulset", "apps-statefulset-v1", "../../docs/catalog/apps-statefulset-v1.yaml").
			AddState(kartav1alpha1.InitializingStatus, ReplicasInitializing()).
			AddState(kartav1alpha1.RunningStatus, FullyAvailable()).
			AddState(kartav1alpha1.DegradedStatus, ReplicasDegraded())
	})

	// A StatefulSet takes transient Initializing/Degraded dips while scaling; the Maybe steps declare them
	// so the order check tolerates them, and the recorder only stops at the ReplicasReady gates.
	It("scaled", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "scaled", "flows/testdata/statefulset/running.yaml").
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).When(ReplicasReady(1)).Do(ScaleReplicas(3)).
			Maybe(kartav1alpha1.InitializingStatus).Maybe(kartav1alpha1.DegradedStatus).
			At(kartav1alpha1.RunningStatus).When(ReplicasReady(3)).Do(ScaleReplicas(1)).
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).WaitUntil(ReplicasReady(1)).Run(ctx)
		Expect(err).To(Succeed())
	})

	// 3 replicas + hostname antiAffinity on 2 nodes: one stays pending, so readyReplicas settles below
	// replicas with updatedReplicas == replicas, read as Degraded.
	It("degraded", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "degraded", "flows/testdata/statefulset/degraded.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.DegradedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
