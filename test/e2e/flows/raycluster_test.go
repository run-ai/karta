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

var _ = Describe("RayCluster", Ordered, Label("kuberay", "raycluster"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/ray-io-raycluster-v1.yaml", "ray-io-raycluster-v1")
		rec = recorder.New("kuberay", "ray-io-raycluster-v1", "../../docs/catalog/ray-io-raycluster-v1.yaml").
			Timeout(8*time.Minute).
			State(Initializing, RayInitializing()).
			State(Running, PhaseEq("ready", "status", "state")).
			State(Suspended, RaySuspended())
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/raycluster/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "cases/testdata/raycluster/suspended.yaml").Reaches(Suspended).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := rec.Flow("resumed", "cases/testdata/raycluster/resumed.yaml").
			At(Suspended).Do(Resume()).Maybe(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})
})
