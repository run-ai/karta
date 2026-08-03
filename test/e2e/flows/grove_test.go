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

var _ = Describe("Grove PodCliqueSet", Ordered, Label("grove"), func() {
	var rec *recorder.Recorder

	BeforeAll(func(ctx SpecContext) {
		installKarta(ctx, "../../docs/catalog/grove-io-podcliqueset-v1alpha1.yaml", "grove-io-podcliqueset-v1alpha1")
		rec = recorder.New("grove", "grove-io-podcliqueset-v1alpha1", "../../docs/catalog/grove-io-podcliqueset-v1alpha1.yaml").
			Timeout(4*time.Minute).
			State(Initializing, ReplicasComingUp()).
			State(Running, AllReplicasAvailable())
	})

	It("running", func(ctx SpecContext) {
		_, err := rec.Flow("running", "cases/testdata/grove/running.yaml").
			Reaches(Initializing).Reaches(Running).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("initializing", func(ctx SpecContext) {
		_, err := rec.Flow("initializing", "cases/testdata/grove/initializing.yaml").Reaches(Initializing).Run(ctx)
		Expect(err).To(Succeed())
	})

	It("scaled", func(ctx SpecContext) {
		_, err := rec.Flow("scaled", "cases/testdata/grove/scaled.yaml").
			Maybe(Initializing).
			At(Running).When(IntEq(1, "status", "availableReplicas")).Do(ScaleReplicas(2)).
			Maybe(Initializing).
			At(Running).When(IntEq(2, "status", "availableReplicas")).Do(ScaleReplicas(1)).
			Maybe(Initializing).
			At(Running).WaitUntil(IntEq(1, "status", "availableReplicas")).Run(ctx)
		Expect(err).To(Succeed())
	})
})
