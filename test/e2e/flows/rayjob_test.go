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

var _ = Describe("RayJob", Ordered, Label("kuberay", "rayjob"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/ray-io-rayjob-v1.yaml", "ray-io-rayjob-v1")
		rec = recorder.New(cfg, "kuberay", operatorVersion("kuberay"), "ray-io-rayjob-v1", "../../docs/catalog/ray-io-rayjob-v1.yaml").
			SetTimeout(6*time.Minute).
			AddState(kartav1alpha1.InitializingStatus, RayJobInitializing()).
			AddState(kartav1alpha1.RunningStatus, PhaseEq("RUNNING", "status", "jobStatus")).
			AddState(kartav1alpha1.CompletedStatus, PhaseEq("SUCCEEDED", "status", "jobStatus")).
			AddState(kartav1alpha1.FailedStatus, PhaseEq("FAILED", "status", "jobStatus")).
			AddState(kartav1alpha1.SuspendedStatus, PhaseAny([]string{"Suspended", "Suspending"}, "status", "jobDeploymentStatus"))
	})

	// jobStatus jumps between PENDING/RUNNING/SUCCEEDED/FAILED; a fast job can skip intermediates, so
	// Initializing and (for terminal flows) Running are Optional.
	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "testdata/rayjob/running.yaml").
			Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "completed", "testdata/rayjob/completed.yaml").
			Maybe(kartav1alpha1.InitializingStatus).Maybe(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.CompletedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "failed", "testdata/rayjob/failed.yaml").
			Maybe(kartav1alpha1.InitializingStatus).Maybe(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.FailedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "suspended", "testdata/rayjob/suspended.yaml").
			Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.SuspendedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "resumed", "testdata/rayjob/resumed.yaml").
			Maybe(kartav1alpha1.InitializingStatus).At(kartav1alpha1.SuspendedStatus).Do(Resume()).Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
