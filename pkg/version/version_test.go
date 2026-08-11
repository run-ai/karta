// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package version

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("version", func() {
	Describe("Get", func() {
		It("returns the compiled-in defaults when nothing is stamped", func() {
			info := Get()
			Expect(info.Version).To(Equal("0.0.0-dev"))
			Expect(info.Commit).To(Equal("unknown"))
			Expect(info.BuildDate).To(Equal("unknown"))
		})

		It("reflects stamped values", func() {
			orig := version
			origCommit := commit
			origBuildDate := buildDate
			DeferCleanup(func() {
				version = orig
				commit = origCommit
				buildDate = origBuildDate
			})

			version = "v1.2.3"
			commit = "abc1234"
			buildDate = "2026-08-10T00:00:00Z"

			info := Get()
			Expect(info.Version).To(Equal("v1.2.3"))
			Expect(info.Commit).To(Equal("abc1234"))
			Expect(info.BuildDate).To(Equal("2026-08-10T00:00:00Z"))
		})

		It("always returns a non-empty GoVersion and Platform", func() {
			info := Get()
			Expect(info.GoVersion).NotTo(BeEmpty())
			Expect(info.Platform).NotTo(BeEmpty())
			Expect(strings.Contains(info.Platform, "/")).To(BeTrue())
		})
	})

	Describe("String", func() {
		It("equals Get().Version", func() {
			Expect(String()).To(Equal(Get().Version))
		})
	})
})
