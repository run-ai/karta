// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("Grove PodCliqueSet", Ordered, Label("grove"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/grove-io-podcliqueset-v1alpha1.yaml", "grove-io-podcliqueset-v1alpha1")
		rec = recorder.New("grove", "grove-io-podcliqueset-v1alpha1", "../../docs/catalog/grove-io-podcliqueset-v1alpha1.yaml").
			SetTimeout(4*time.Minute).
			AddState(kartav1alpha1.InitializingStatus, ReplicasComingUp()).
			AddState(kartav1alpha1.RunningStatus, AllReplicasAvailable())
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "flows/testdata/grove/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "initializing", "flows/testdata/grove/initializing.yaml").Reaches(kartav1alpha1.InitializingStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("scaled", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "scaled", "flows/testdata/grove/scaled.yaml").
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).When(IntEq(1, "status", "availableReplicas")).Do(ScaleReplicas(2)).
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).When(IntEq(2, "status", "availableReplicas")).Do(ScaleReplicas(1)).
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).WaitUntil(IntEq(1, "status", "availableReplicas")).Run(ctx)
		Expect(err).To(Succeed())
	})
})
