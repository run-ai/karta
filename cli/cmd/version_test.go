// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/run-ai/karta/pkg/version"
)

var _ = Describe("version command", func() {
	var dir string

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		for _, key := range []string{"KARTA_CONFIG", "KARTA_OUTPUT"} {
			DeferCleanup(os.Setenv, key, os.Getenv(key))
			Expect(os.Unsetenv(key)).To(Succeed())
		}
	})

	DescribeTable("output formats",
		func(args []string, check func(string)) {
			out, err := runVersion(args...)
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(BeEmpty())
			check(out)
		},
		Entry("table", []string{"version"}, func(out string) {
			Expect(out).To(ContainSubstring("Version:"))
			Expect(out).To(ContainSubstring("Go Version:"))
			Expect(out).To(ContainSubstring("Platform:"))
		}),
		Entry("wide", []string{"version", "-o", "wide"}, func(out string) {
			Expect(out).To(ContainSubstring("Version:"))
			Expect(out).To(ContainSubstring("Go Version:"))
			Expect(out).To(ContainSubstring("Platform:"))
		}),
		Entry("json", []string{"version", "-o", "json"}, func(out string) {
			var info version.Info
			Expect(json.Unmarshal([]byte(out), &info)).To(Succeed())
			Expect(info.GoVersion).NotTo(BeEmpty())
			Expect(info.Platform).NotTo(BeEmpty())
		}),
		Entry("yaml", []string{"version", "-o", "yaml"}, func(out string) {
			Expect(out).To(ContainSubstring("goVersion:"))
			Expect(out).To(ContainSubstring("platform:"))
		}),
	)

	It("table and wide produce identical output", func() {
		table, err := runVersion("version")
		Expect(err).NotTo(HaveOccurred())
		wide, err := runVersion("version", "-o", "wide")
		Expect(err).NotTo(HaveOccurred())
		Expect(table).To(Equal(wide))
	})

	It("rejects positional arguments", func() {
		_, err := runVersion("version", "unexpected")
		Expect(err).To(HaveOccurred())
	})

	It("succeeds with a malformed config while other commands fail", func() {
		cfg := filepath.Join(dir, "bad.yaml")
		writeFile(cfg, "output: [not-a-string\n")

		_, err := runVersion("--config", cfg, "version")
		Expect(err).NotTo(HaveOccurred(), "version should succeed even when config is malformed")

		// A command that inherits the root PersistentPreRunE must fail on bad config.
		root := NewRootCommand()
		root.AddCommand(&cobra.Command{
			Use:  "noop",
			RunE: func(*cobra.Command, []string) error { return nil },
		})
		root.SetArgs([]string{"--config", cfg, "noop"})
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		Expect(root.Execute()).To(HaveOccurred())
	})
})

func runVersion(args ...string) (string, error) {
	GinkgoHelper()
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}
