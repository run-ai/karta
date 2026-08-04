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

var _ = Describe("KnativeService", Ordered, Label("knative"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/serving-knative-dev-service-v1.yaml", "serving-knative-dev-service-v1")
		rec = recorder.New(cfg, "knative", operatorVersion("knative"), "serving-knative-dev-service-v1", "../../docs/catalog/serving-knative-dev-service-v1.yaml").
			SetTimeout(5*time.Minute).
			AddState(kartav1alpha1.InitializingStatus, CondStatus("Ready", "Unknown")).
			AddState(kartav1alpha1.RunningStatus, CondTrue("Ready"))
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "testdata/knative/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
