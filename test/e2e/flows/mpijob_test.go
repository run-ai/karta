// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("MPIJob", Ordered, Label("kubeflow", "mpijob"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/kubeflow-org-mpijob-v2beta1.yaml", "kubeflow-org-mpijob-v2beta1")
		rec = recorder.New("kubeflow", "kubeflow-org-mpijob-v2beta1", "../../docs/catalog/kubeflow-org-mpijob-v2beta1.yaml").
			State(Initializing, CondTrue("Created")).
			State(Running, CondTrue("Running")).
			State(Completed, CondTrue("Succeeded")).
			State(Failed, CondTrue("Failed")).
			State(Suspended, CondTrue("Suspended"))
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/mpijob/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	// The launcher can finish before Running is observed (Optional), and Kubeflow keeps Created set so the
	// CR reads Initializing again for a tick before the terminal.
	It("completed", func(ctx SpecContext) {
		_, err := rec.Flow("completed", "cases/testdata/mpijob/completed.yaml").
			Reaches(Initializing).Maybe(Running).Reaches(Initializing).Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := rec.Flow("failed", "cases/testdata/mpijob/failed.yaml").
			Reaches(Initializing).Maybe(Running).Reaches(Initializing).Reaches(Failed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "cases/testdata/mpijob/suspended.yaml").
			Reaches(Suspended).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := rec.Flow("resumed", "cases/testdata/mpijob/resumed.yaml").
			At(Suspended).Do(ResumeRunPolicy()).
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})
})
