// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("CronJob (built-in)", Ordered, Label("cronjob", "builtin"), func() {
	var rec *recorder.Recorder
	var fx recorder.Fixture

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/batch-cronjob-v1.yaml", "batch-cronjob-v1")
		fx = recorder.Fixture{Operator: "cronjob", Version: operatorVersion("cronjob"), KartaName: "batch-cronjob-v1", KartaFile: "../../docs/catalog/batch-cronjob-v1.yaml"}
		rec = recorder.New(cfg).
			AddState(kartav1alpha1.InitializingStatus, Absent("status", "lastScheduleTime")).
			AddState(kartav1alpha1.RunningStatus, CronjobFired()).
			AddState(kartav1alpha1.SuspendedStatus, BoolTrue("spec", "suspend"))
	})

	It("initializing", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "initializing", "testdata/cronjob/initializing.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})

	It("running", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "running", "testdata/cronjob/running.yaml").
			Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "suspended", "testdata/cronjob/suspended.yaml").
			Reaches(kartav1alpha1.SuspendedStatus).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})
})
