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

var _ = Describe("RayCluster", Ordered, Label("kuberay", "raycluster"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/ray-io-raycluster-v1.yaml", "ray-io-raycluster-v1")
		rec = recorder.New("kuberay", "ray-io-raycluster-v1", "../../docs/catalog/ray-io-raycluster-v1.yaml").
			Timeout(8*time.Minute).
			State(kartav1alpha1.InitializingStatus, RayInitializing()).
			State(kartav1alpha1.RunningStatus, PhaseEq("ready", "status", "state")).
			State(kartav1alpha1.SuspendedStatus, RaySuspended())
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "flows/testdata/raycluster/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "flows/testdata/raycluster/suspended.yaml").Reaches(kartav1alpha1.SuspendedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("resumed", func(ctx SpecContext) {
		_, err := rec.Flow("resumed", "flows/testdata/raycluster/resumed.yaml").
			At(kartav1alpha1.SuspendedStatus).Do(Resume()).Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
