// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("JobSet", Ordered, Label("jobset"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml", "jobset-x-k8s-io-jobset-v1alpha2")
		// Suspended first so a lingering Suspended condition never masks real progress after a resume.
		rec = recorder.New(cluster, "jobset", "jobset-x-k8s-io-jobset-v1alpha2", "../../docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml").
			AddState(kartav1alpha1.SuspendedStatus, CondTrue("Suspended")).
			AddState(kartav1alpha1.InitializingStatus, JobsetInitializing()).
			AddState(kartav1alpha1.RunningStatus, JobsetRunning()).
			AddState(kartav1alpha1.CompletedStatus, CondTrue("Completed")).
			AddState(kartav1alpha1.FailedStatus, CondTrue("Failed"))
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "flows/testdata/jobset/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "completed", "flows/testdata/jobset/completed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.CompletedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "failed", "flows/testdata/jobset/failed.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Maybe(kartav1alpha1.RunningStatus).Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.FailedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "resumed", "flows/testdata/jobset/resumed.yaml").
			Maybe(kartav1alpha1.InitializingStatus).At(kartav1alpha1.SuspendedStatus).Do(Resume()).
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.CompletedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "suspended", "flows/testdata/jobset/suspended.yaml").
			Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.SuspendedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
