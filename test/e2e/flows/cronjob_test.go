// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("CronJob (built-in)", Ordered, Label("cronjob", "builtin"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/batch-cronjob-v1.yaml", "batch-cronjob-v1")
		rec = recorder.New("cronjob", "batch-cronjob-v1", "../../docs/catalog/batch-cronjob-v1.yaml").
			State(Initializing, Absent("status", "lastScheduleTime")).
			State(Running, CronjobFired()).
			State(Suspended, BoolTrue("spec", "suspend"))
	})

	It("initializing", func(ctx SpecContext) {
		_, err := rec.Flow("initializing", "cases/testdata/cronjob/initializing.yaml").
			Reaches(Initializing).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/cronjob/running.yaml").
			Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("suspended", func(ctx SpecContext) {
		_, err := rec.Flow("suspended", "cases/testdata/cronjob/suspended.yaml").
			Reaches(Suspended).Run(ctx)
		Expect(err).To(Succeed())
	})
})
