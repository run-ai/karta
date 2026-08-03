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

var _ = Describe("RayJob", Ordered, Label("kuberay", "rayjob"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/ray-io-rayjob-v1.yaml", "ray-io-rayjob-v1")
		rec = recorder.New("kuberay", "ray-io-rayjob-v1", "../../docs/catalog/ray-io-rayjob-v1.yaml").
			Timeout(6*time.Minute).
			State(Initializing, RayJobInitializing()).
			State(Running, PhaseEq("RUNNING", "status", "jobStatus")).
			State(Completed, PhaseEq("SUCCEEDED", "status", "jobStatus")).
			State(Failed, PhaseEq("FAILED", "status", "jobStatus")).
			State(Suspended, PhaseEq("Suspended", "status", "jobDeploymentStatus"))
	})

	// jobStatus jumps between PENDING/RUNNING/SUCCEEDED/FAILED; a fast job can skip intermediates, so
	// Initializing and (for terminal flows) Running are Optional.
	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/rayjob/running.yaml").
			Maybe(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("completed", func(ctx SpecContext) {
		_, err := rec.Flow("completed", "cases/testdata/rayjob/completed.yaml").
			Maybe(Initializing).Maybe(Running).Reaches(Completed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("failed", func(ctx SpecContext) {
		_, err := rec.Flow("failed", "cases/testdata/rayjob/failed.yaml").
			Maybe(Initializing).Maybe(Running).Reaches(Failed).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "cases/testdata/rayjob/suspended.yaml").Reaches(Suspended).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := rec.Flow("resumed", "cases/testdata/rayjob/resumed.yaml").
			At(Suspended).Do(Resume()).Maybe(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})
})
