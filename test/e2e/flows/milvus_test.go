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

var _ = Describe("Milvus", Ordered, Label("milvus"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/milvus-io-milvus-v1beta1.yaml", "milvus-io-milvus-v1beta1")
		rec = recorder.New("milvus", "milvus-io-milvus-v1beta1", "../../docs/catalog/milvus-io-milvus-v1beta1.yaml").
			Timeout(8*time.Minute).
			State(Initializing, PhaseEq("Pending", "status", "status")).
			State(Running, CondTrue("MilvusReady"))
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/milvus/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := rec.Flow("initializing", "cases/testdata/milvus/initializing.yaml").Reaches(Initializing).Run(ctx)
		Expect(err).To(Succeed())
	})
})
