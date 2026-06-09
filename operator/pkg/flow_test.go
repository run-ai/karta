// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StepResult helpers", func() {
	It("Continue does not short-circuit and carries no error", func() {
		r := Continue()
		Expect(r.ShortCircuit()).To(BeFalse())
		_, err := r.Result()
		Expect(err).NotTo(HaveOccurred())
	})

	It("StopWithError short-circuits with the error and requests requeue", func() {
		sentinel := errors.New("step failed")
		r := StopWithError(sentinel)
		Expect(r.ShortCircuit()).To(BeTrue())
		res, err := r.Result()
		Expect(errors.Is(err, sentinel)).To(BeTrue())
		Expect(res.Requeue).To(BeTrue())
	})
})
