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

var _ = Describe("NIMService", Ordered, Label("nim"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/apps-nvidia-com-nimservice-v1alpha1.yaml", "apps-nvidia-com-nimservice-v1alpha1")
		// The NIMService references authSecret ngc-secret; the operator reads it from the workload's own
		// namespace, so seed it here (up.sh only creates it in default).
		ensureSecret(ctx, "ngc-secret", map[string]string{"NGC_API_KEY": "dummy-not-a-real-token"})
		rec = recorder.New("nim", "apps-nvidia-com-nimservice-v1alpha1", "../../docs/catalog/apps-nvidia-com-nimservice-v1alpha1.yaml").
			Timeout(5*time.Minute).
			State(Initializing, PhaseNot([]string{"Ready", "Failed"}, "status", "state")).
			State(Running, PhaseEq("Ready", "status", "state"))
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/nim/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := rec.Flow("initializing", "cases/testdata/nim/initializing.yaml").Reaches(Initializing).Run(ctx)
		Expect(err).To(Succeed())
	})
})
