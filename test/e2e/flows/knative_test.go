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

var _ = Describe("KnativeService", Ordered, Label("knative"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/serving-knative-dev-service-v1.yaml", "serving-knative-dev-service-v1")
		rec = recorder.New("knative", "serving-knative-dev-service-v1", "../../docs/catalog/serving-knative-dev-service-v1.yaml").
			Timeout(5*time.Minute).
			State(Running, CondTrue("Ready"))
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/knative/running.yaml").Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})
})
