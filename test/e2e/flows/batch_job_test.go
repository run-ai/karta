// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("BatchJob (built-in)", Ordered, Label("batch-job", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/batch-job-v1.yaml", "batch-job-v1")
		rec = recorder.New("batch-job", "batch-job-v1", "../../docs/catalog/batch-job-v1.yaml").
			State(Suspended, CondTrue("Suspended")).
			State(Initializing, IntAtLeast(1, "status", "active")).
			State(Running, IntAtLeast(1, "status", "ready")).
			State(Completed, CondTrue("Complete", "SuccessCriteriaMet")).
			State(Failed, CondTrue("Failed", "FailureTarget")).
			State(Degraded, JobDegraded())
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/batch-job/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := rec.Flow("completed", "cases/testdata/batch-job/completed.yaml").
			Reaches(Initializing).Reaches(Running).
			Reaches(Initializing). // active-not-ready dip as the pod terminates
			Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := rec.Flow("failed", "cases/testdata/batch-job/failed.yaml").
			Reaches(Initializing).Reaches(Failed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := rec.Flow("resumed", "cases/testdata/batch-job/resumed.yaml").
			At(Suspended).Do(Resume()).
			Reaches(Initializing).Reaches(Running).Reaches(Initializing).Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("degraded", func(ctx SpecContext) {
		_, err := rec.Flow("degraded", "cases/testdata/batch-job/degraded.yaml").
			Reaches(Initializing).Reaches(Running).Reaches(Degraded).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "cases/testdata/batch-job/suspended.yaml").
			Reaches(Suspended).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("scaled", func(ctx SpecContext) {
		_, err := rec.Flow("scaled", "cases/testdata/batch-job/scaled.yaml").
			Maybe(Initializing).
			At(Running).When(IntEq(1, "status", "ready")).Do(ScaleParallelism(3)).
			At(Running).When(IntEq(3, "status", "ready")).Do(ScaleParallelism(1)).
			At(Running).WaitUntil(IntEq(1, "status", "ready")).Run(ctx)
		Expect(err).To(Succeed())
	})
})
