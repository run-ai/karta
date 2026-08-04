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

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/batch-cronjob-v1.yaml", "batch-cronjob-v1")
		rec = recorder.New(cluster, "cronjob", "batch-cronjob-v1", "../../docs/catalog/batch-cronjob-v1.yaml").
			AddState(kartav1alpha1.InitializingStatus, Absent("status", "lastScheduleTime")).
			AddState(kartav1alpha1.RunningStatus, CronjobFired()).
			AddState(kartav1alpha1.SuspendedStatus, BoolTrue("spec", "suspend"))
	})

	It("initializing", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "initializing", "flows/testdata/cronjob/initializing.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "flows/testdata/cronjob/running.yaml").
			Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "suspended", "flows/testdata/cronjob/suspended.yaml").
			Reaches(kartav1alpha1.SuspendedStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
