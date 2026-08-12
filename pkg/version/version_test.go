// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package version

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("version", func() {
	Describe("String", func() {
		It("returns the compiled-in default when nothing is stamped", func() {
			Expect(String()).To(Equal("0.0.0-dev"))
		})

		It("reflects the value stamped at link time", func() {
			orig := version
			DeferCleanup(func() { version = orig })

			version = "1.2.3"
			Expect(String()).To(Equal("1.2.3"))
		})
	})
})
