// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"bytes"
	"io"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/run-ai/karta/pkg/version"
)

var _ = Describe("--version", func() {
	It("prints the stamped version and nothing else", func() {
		out, err := runKarta("--version")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(version.String() + "\n"))
	})

	It("succeeds with a malformed config while other commands fail", func() {
		cfg := filepath.Join(GinkgoT().TempDir(), "bad.yaml")
		writeFile(cfg, "output: [not-a-string\n")

		out, err := runKarta("--config", cfg, "--version")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(version.String() + "\n"))

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

func runKarta(args ...string) (string, error) {
	GinkgoHelper()
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}
