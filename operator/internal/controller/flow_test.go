// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StepResult helpers", func() {
	It("Continue does not short-circuit and carries no error", func() {
		r := Continue()
		Expect(shortCircuit(r)).To(BeFalse())
		_, err := r.Result()
		Expect(err).NotTo(HaveOccurred())
	})

	It("Stop short-circuits with no error and no requeue", func() {
		r := Stop()
		Expect(shortCircuit(r)).To(BeTrue())
		res, err := r.Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("StopWithError short-circuits with the error and requests requeue", func() {
		sentinel := errors.New("step failed")
		r := StopWithError(sentinel)
		Expect(shortCircuit(r)).To(BeTrue())
		res, err := r.Result()
		Expect(errors.Is(err, sentinel)).To(BeTrue())
		Expect(res.Requeue).To(BeTrue())
	})
})
