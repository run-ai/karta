// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("Deployment (built-in)", Ordered, Label("deployment", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/apps-deployment-v1.yaml", "apps-deployment-v1")
		rec = recorder.New(cluster, "deployment", "apps-deployment-v1", "../../docs/catalog/apps-deployment-v1.yaml").
			AddState(kartav1alpha1.InitializingStatus, AllOf(CondTrue("Progressing"), CondNotTrue("Available"))).
			AddState(kartav1alpha1.RunningStatus, CondReason("Progressing", "NewReplicaSetAvailable")).
			AddState(kartav1alpha1.FailedStatus, CondFalse("Progressing"))
	})

	It("scaled", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "scaled", "flows/testdata/deployment/running.yaml").
			Maybe(kartav1alpha1.InitializingStatus). // startup, before the first Running (Deployment stays Running while scaling)
			At(kartav1alpha1.RunningStatus).When(ReplicasReady(1)).Do(ScaleReplicas(3)).
			At(kartav1alpha1.RunningStatus).When(ReplicasReady(3)).Do(ScaleReplicas(1)).
			At(kartav1alpha1.RunningStatus).WaitUntil(ReplicasReady(1)).Run(ctx)
		Expect(err).To(Succeed())
	})

	// Bad image, no progress deadline: Progressing stays True with Available False, read as Initializing.
	It("initializing", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "initializing", "flows/testdata/deployment/initializing.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	// Pinned to a nonexistent node with a 10s progress deadline: Progressing=False/ProgressDeadlineExceeded,
	// read as Failed. It passes through Initializing first.
	It("failed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "failed", "flows/testdata/deployment/failed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.FailedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
