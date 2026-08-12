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
	It("reports the version even when the config is malformed", func() {
		cfg := filepath.Join(GinkgoT().TempDir(), "bad.yaml")
		writeFile(cfg, "output: [not-a-string\n")

		root := NewRootCommand()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs([]string{"--config", cfg, "--version"})
		Expect(root.Execute()).To(Succeed())
		Expect(buf.String()).To(Equal(version.String() + "\n"))

		// A command that inherits the root PersistentPreRunE must fail on the same config.
		other := NewRootCommand()
		other.AddCommand(&cobra.Command{
			Use:  "noop",
			RunE: func(*cobra.Command, []string) error { return nil },
		})
		other.SetArgs([]string{"--config", cfg, "noop"})
		other.SetOut(io.Discard)
		other.SetErr(io.Discard)
		Expect(other.Execute()).To(HaveOccurred())
	})
})
