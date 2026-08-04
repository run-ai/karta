// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("BatchJob (built-in)", Ordered, Label("batch-job", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/batch-job-v1.yaml", "batch-job-v1")
		rec = recorder.New(cfg, "batch-job", operatorVersion("batch-job"), "batch-job-v1", "../../docs/catalog/batch-job-v1.yaml").
			AddState(kartav1alpha1.SuspendedStatus, CondTrue("Suspended")).
			AddState(kartav1alpha1.InitializingStatus, IntAtLeast(1, "status", "active")).
			AddState(kartav1alpha1.RunningStatus, IntAtLeast(1, "status", "ready")).
			AddState(kartav1alpha1.CompletedStatus, CondTrue("Complete", "SuccessCriteriaMet")).
			AddState(kartav1alpha1.FailedStatus, CondTrue("Failed", "FailureTarget")).
			AddState(kartav1alpha1.DegradedStatus, JobDegraded())
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "testdata/batch-job/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "completed", "testdata/batch-job/completed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).
			Reaches(kartav1alpha1.InitializingStatus). // active-not-ready dip as the pod terminates
			Reaches(kartav1alpha1.CompletedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "failed", "testdata/batch-job/failed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.FailedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "resumed", "testdata/batch-job/resumed.yaml").
			At(kartav1alpha1.SuspendedStatus).Do(Resume()).
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.CompletedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("degraded", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "degraded", "testdata/batch-job/degraded.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Reaches(kartav1alpha1.DegradedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "suspended", "testdata/batch-job/suspended.yaml").
			Reaches(kartav1alpha1.SuspendedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("scaled", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "scaled", "testdata/batch-job/scaled.yaml").
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).When(IntEq(1, "status", "ready")).Do(ScaleParallelism(3)).
			At(kartav1alpha1.RunningStatus).When(IntEq(3, "status", "ready")).Do(ScaleParallelism(1)).
			At(kartav1alpha1.RunningStatus).WaitUntil(IntEq(1, "status", "ready")).Run(ctx)
		Expect(err).To(Succeed())
	})
})
