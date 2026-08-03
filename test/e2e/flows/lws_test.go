// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("LeaderWorkerSet", Ordered, Label("lws"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml", "leaderworkerset-x-k8s-io-leaderworkerset-v1")
		rec = recorder.New("lws", "leaderworkerset-x-k8s-io-leaderworkerset-v1", "../../docs/catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml").
			State(Initializing, AllOf(CondTrue("Progressing"), CondFalse("Available"))).
			State(Running, ReplicasSettled())
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/lws/running.yaml").
			Maybe(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("scaled", func(ctx SpecContext) {
		_, err := rec.Flow("scaled", "cases/testdata/lws/scaled.yaml").
			Maybe(Initializing).
			At(Running).When(ReplicasReady(1)).Do(ScaleReplicas(2)).
			Maybe(Initializing).
			At(Running).When(ReplicasReady(2)).Do(ScaleReplicas(1)).
			Maybe(Initializing).
			At(Running).WaitUntil(ReplicasReady(1)).Run(ctx)
		Expect(err).To(Succeed())
	})
})
