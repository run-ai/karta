// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("Pod (built-in)", Ordered, Label("pod", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/core-pod-v1.yaml", "core-pod-v1")
		rec = recorder.New(cfg, "pod", operatorVersion("pod"), "core-pod-v1", "../../docs/catalog/core-pod-v1.yaml").
			AddState(kartav1alpha1.InitializingStatus, PhaseEq("Pending", "status", "phase")).
			AddState(kartav1alpha1.RunningStatus, PhaseEq("Running", "status", "phase")).
			AddState(kartav1alpha1.CompletedStatus, PhaseEq("Succeeded", "status", "phase")).
			AddState(kartav1alpha1.FailedStatus, PhaseEq("Failed", "status", "phase"))
	})

	It("happy", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "happy", "testdata/pod/happy.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "completed", "testdata/pod/completed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.CompletedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "failed", "testdata/pod/failed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.FailedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "initializing", "testdata/pod/initializing.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
