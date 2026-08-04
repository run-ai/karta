// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("NIMService", Ordered, Label("nim"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/apps-nvidia-com-nimservice-v1alpha1.yaml", "apps-nvidia-com-nimservice-v1alpha1")
		// The NIMService references authSecret ngc-secret; the operator reads it from the workload's own
		// namespace, so seed it here (up.sh only creates it in default).
		ensureSecret(ctx, "ngc-secret", map[string]string{"NGC_API_KEY": "dummy-not-a-real-token"})
		rec = recorder.New(cluster, "nim", operatorVersion("nim"), "apps-nvidia-com-nimservice-v1alpha1", "../../docs/catalog/apps-nvidia-com-nimservice-v1alpha1.yaml").
			SetTimeout(5*time.Minute).
			AddState(kartav1alpha1.InitializingStatus, PhaseNot([]string{"Ready", "Failed"}, "status", "state")).
			AddState(kartav1alpha1.RunningStatus, PhaseEq("Ready", "status", "state"))
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "testdata/nim/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "initializing", "testdata/nim/initializing.yaml").Reaches(kartav1alpha1.InitializingStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
