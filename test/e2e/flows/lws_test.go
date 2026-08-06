// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/recorder"
)

var _ = Describe("LeaderWorkerSet", Ordered, Label("lws"), func() {
	var rec *recorder.Recorder
	var fx recorder.Fixture

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml", "leaderworkerset-x-k8s-io-leaderworkerset-v1")
		fx = recorder.Fixture{Operator: "lws", Version: operatorVersion("lws"), KartaName: "leaderworkerset-x-k8s-io-leaderworkerset-v1", KartaFile: "../../docs/catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml"}
		rec = recorder.New(cfg).
			AddState(kartav1alpha1.InitializingStatus, AllOf(CondTrue("Progressing"), CondNotTrue("Available"))).
			AddState(kartav1alpha1.RunningStatus, CondTrue("Available"))
	})

	It("running", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "running", "testdata/lws/running.yaml").
			Maybe(kartav1alpha1.InitializingStatus).Reaches(kartav1alpha1.RunningStatus).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})

	It("scaled", func(ctx SpecContext) {
		out, err := recorder.NewFlow(rec, "scaled", "testdata/lws/scaled.yaml").
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).When(ReplicasReady(1)).Do(ScaleReplicas(2)).
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).When(ReplicasReady(2)).Do(ScaleReplicas(1)).
			Maybe(kartav1alpha1.InitializingStatus).
			At(kartav1alpha1.RunningStatus).WaitUntil(ReplicasReady(1)).Run(ctx)
		Expect(rec.Save(fx, out)).Error().NotTo(HaveOccurred())
		Expect(err).To(Succeed())
	})
})
