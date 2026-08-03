// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("Pod (built-in)", Ordered, Label("pod", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/core-pod-v1.yaml", "core-pod-v1")
		rec = recorder.New("pod", "core-pod-v1", "../../docs/catalog/core-pod-v1.yaml").
			State(Initializing, PhaseEq("Pending", "status", "phase")).
			State(Running, PhaseEq("Running", "status", "phase")).
			State(Completed, PhaseEq("Succeeded", "status", "phase")).
			State(Failed, PhaseEq("Failed", "status", "phase"))
	})

	It("happy", func(ctx SpecContext) {
		_, err := rec.Flow("happy", "cases/testdata/pod/happy.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := rec.Flow("completed", "cases/testdata/pod/completed.yaml").
			Reaches(Initializing).Reaches(Running).Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := rec.Flow("failed", "cases/testdata/pod/failed.yaml").
			Reaches(Initializing).Reaches(Running).Reaches(Failed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := rec.Flow("initializing", "cases/testdata/pod/initializing.yaml").
			Reaches(Initializing).Run(ctx)
		Expect(err).To(Succeed())
	})
})
