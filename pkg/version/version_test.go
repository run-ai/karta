// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package version

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("version", func() {
	// The default is mirrored by the Makefiles and the operator Dockerfile, and
	// is the sentinel cli-verify-version checks against.
	It("defaults to 0.0.0-dev when the binary is not stamped", func() {
		Expect(String()).To(Equal("0.0.0-dev"))
	})
})
