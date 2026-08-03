// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("PyTorchJob", Ordered, Label("kubeflow", "pytorch"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/kubeflow-org-pytorchjob-v1.yaml", "kubeflow-org-pytorchjob-v1")
		rec = recorder.New("kubeflow", "kubeflow-org-pytorchjob-v1", "../../docs/catalog/kubeflow-org-pytorchjob-v1.yaml").
			Timeout(4*time.Minute).
			State(Initializing, CondTrue("Created")).
			State(Running, CondTrue("Running")).
			State(Completed, CondTrue("Succeeded")).
			State(Failed, CondTrue("Failed")).
			State(Suspended, CondTrue("Suspended"))
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/pytorch/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := rec.Flow("completed", "cases/testdata/pytorch/completed.yaml").
			Reaches(Initializing).Maybe(Running).Reaches(Initializing).Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := rec.Flow("failed", "cases/testdata/pytorch/failed.yaml").
			Reaches(Initializing).Maybe(Running).Reaches(Initializing).Reaches(Failed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "cases/testdata/pytorch/suspended.yaml").
			Reaches(Suspended).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := rec.Flow("resumed", "cases/testdata/pytorch/resumed.yaml").
			At(Suspended).Do(ResumeRunPolicy()).
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})
})
