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

var _ = Describe("PyTorchJob", Ordered, Label("kubeflow", "pytorch"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/kubeflow-org-pytorchjob-v1.yaml", "kubeflow-org-pytorchjob-v1")
		rec = recorder.New(cluster, "kubeflow", "kubeflow-org-pytorchjob-v1", "../../docs/catalog/kubeflow-org-pytorchjob-v1.yaml").
			SetTimeout(4*time.Minute).
			AddState(kartav1alpha1.InitializingStatus, CondTrue("Created")).
			AddState(kartav1alpha1.RunningStatus, CondTrue("Running")).
			AddState(kartav1alpha1.CompletedStatus, CondTrue("Succeeded")).
			AddState(kartav1alpha1.FailedStatus, CondTrue("Failed")).
			AddState(kartav1alpha1.SuspendedStatus, CondTrue("Suspended"))
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "flows/testdata/pytorch/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "completed", "flows/testdata/pytorch/completed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Maybe(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.CompletedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "failed", "flows/testdata/pytorch/failed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Maybe(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.FailedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "suspended", "flows/testdata/pytorch/suspended.yaml").
			Reaches(kartav1alpha1.SuspendedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "resumed", "flows/testdata/pytorch/resumed.yaml").
			At(kartav1alpha1.SuspendedStatus).Do(ResumeRunPolicy()).
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
