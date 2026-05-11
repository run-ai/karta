// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

import (
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

var _ = Describe("docs/examples", func() {
	var examplesDir string
	var exampleFiles []string

	BeforeEach(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		Expect(ok).To(BeTrue())
		examplesDir = filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "docs", "examples")

		matches, err := filepath.Glob(filepath.Join(examplesDir, "*.yaml"))
		Expect(err).NotTo(HaveOccurred())
		exampleFiles = matches
	})

	It("contains example files", func() {
		Expect(exampleFiles).NotTo(BeEmpty(), "no example YAMLs found in %s", examplesDir)
	})

	It("each example is a valid Karta", func() {
		for _, path := range exampleFiles {
			By(filepath.Base(path))

			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred(), "read %s", path)

			var karta Karta
			Expect(yaml.Unmarshal(data, &karta)).To(Succeed(), "unmarshal %s", path)
			Expect(karta.APIVersion).To(Equal("run.ai/v1alpha1"), "apiVersion in %s", path)
			Expect(karta.Kind).To(Equal("Karta"), "kind in %s", path)

			Expect(NewKartaValidator(&karta).Validate()).To(Succeed(), "validate %s", path)
		}
	})
})
