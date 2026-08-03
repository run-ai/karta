// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("JobSet", Label("jobset"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml", "jobset-x-k8s-io-jobset-v1alpha2")
		// Suspended first so a lingering Suspended condition never masks real progress after a resume.
		rec = recorder.New("jobset", "jobset-x-k8s-io-jobset-v1alpha2", "../../docs/catalog/jobset-x-k8s-io-jobset-v1alpha2.yaml").
			State(Suspended, CondTrue("Suspended")).
			State(Initializing, JobsetInitializing()).
			State(Running, JobsetRunning()).
			State(Completed, CondTrue("Completed")).
			State(Failed, CondTrue("Failed"))
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/jobset/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := rec.Flow("completed", "cases/testdata/jobset/completed.yaml").
			Reaches(Initializing).Reaches(Running).Reaches(Initializing).Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := rec.Flow("failed", "cases/testdata/jobset/failed.yaml").
			Reaches(Initializing).Reaches(Failed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := rec.Flow("resumed", "cases/testdata/jobset/resumed.yaml").
			At(Suspended).Do(Resume()).
			Reaches(Initializing).Reaches(Running).Reaches(Initializing).Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "cases/testdata/jobset/suspended.yaml").
			Reaches(Suspended).Run(ctx)
		Expect(err).To(Succeed())
	})
})
