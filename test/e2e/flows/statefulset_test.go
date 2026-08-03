// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("StatefulSet (built-in)", Ordered, Label("statefulset", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/apps-statefulset-v1.yaml", "apps-statefulset-v1")
		rec = recorder.New("statefulset", "apps-statefulset-v1", "../../docs/catalog/apps-statefulset-v1.yaml").
			State(Initializing, ReplicasInitializing()).
			State(Running, FullyAvailable()).
			State(Degraded, ReplicasDegraded())
	})

	// A StatefulSet takes transient Initializing/Degraded dips while scaling; the Maybe steps declare them
	// so the order check tolerates them, and the recorder only stops at the ReplicasReady gates.
	It("scaled", func(ctx SpecContext) {
		_, err := rec.Flow("scaled", "cases/testdata/statefulset/running.yaml").
			Maybe(Initializing).
			At(Running).When(ReplicasReady(1)).Do(ScaleReplicas(3)).
			Maybe(Initializing).Maybe(Degraded).
			At(Running).When(ReplicasReady(3)).Do(ScaleReplicas(1)).
			Maybe(Initializing).
			At(Running).WaitUntil(ReplicasReady(1)).Run(ctx)
		Expect(err).To(Succeed())
	})

	// 3 replicas + hostname antiAffinity on 2 nodes: one stays pending, so readyReplicas settles below
	// replicas with updatedReplicas == replicas, read as Degraded.
	It("degraded", func(ctx SpecContext) {
		_, err := rec.Flow("degraded", "cases/testdata/statefulset/degraded.yaml").
			Reaches(Initializing).Reaches(Degraded).Run(ctx)
		Expect(err).To(Succeed())
	})
})
