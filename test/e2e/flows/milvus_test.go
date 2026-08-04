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

var _ = Describe("Milvus", Ordered, Label("milvus"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/milvus-io-milvus-v1beta1.yaml", "milvus-io-milvus-v1beta1")
		rec = recorder.New("milvus", "milvus-io-milvus-v1beta1", "../../docs/catalog/milvus-io-milvus-v1beta1.yaml").
			SetTimeout(8*time.Minute).
			AddState(kartav1alpha1.InitializingStatus, PhaseAny([]string{"Pending", ""}, "status", "status")).
			AddState(kartav1alpha1.RunningStatus, CondTrue("MilvusReady"))
	})

	It("running", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "running", "flows/testdata/milvus/running.yaml").
			Reaches(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := recorder.NewFlow(rec, "initializing", "flows/testdata/milvus/initializing.yaml").Reaches(kartav1alpha1.InitializingStatus).Run(ctx)
		Expect(err).To(Succeed())
	})
})
