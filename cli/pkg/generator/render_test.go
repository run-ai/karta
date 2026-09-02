// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator_test

import (
	"bytes"
	"errors"
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/run-ai/karta/cli/pkg/generator"
)

type item struct {
	Name string `json:"name"`
}

var items = []item{{Name: "alpha"}, {Name: "beta"}}

// unusedTable fails the spec if a machine format reaches the table callback.
func unusedTable(io.Writer) error {
	Fail("the table callback must not run for a machine format")
	return nil
}

var _ = Describe("Render", func() {
	DescribeTable("hands the human formats to the table callback",
		func(format generator.Output) {
			var out bytes.Buffer
			called := false
			Expect(generator.Render(&out, format, items, func(w io.Writer) error {
				called = true
				_, err := io.WriteString(w, "TABLE")
				return err
			})).To(Succeed())

			Expect(called).To(BeTrue())
			Expect(out.String()).To(Equal("TABLE"))
		},
		Entry("table", generator.OutputTable),
		// wide reaches the callback so a command that renders extra columns can.
		// One that cannot must reject wide before calling Render.
		Entry("wide", generator.OutputWide),
	)

	It("returns what the table callback returns", func() {
		boom := errors.New("boom")
		err := generator.Render(io.Discard, generator.OutputTable, items,
			func(io.Writer) error { return boom })
		Expect(err).To(MatchError(boom))
	})

	It("encodes json through the json tags", func() {
		var out bytes.Buffer
		Expect(generator.Render(&out, generator.OutputJSON, items, unusedTable)).To(Succeed())
		Expect(out.String()).To(ContainSubstring(`"name": "alpha"`))
	})

	It("emits an empty json array for no items, not null", func() {
		var out bytes.Buffer
		Expect(generator.Render[item](&out, generator.OutputJSON, nil, unusedTable)).To(Succeed())
		Expect(strings.TrimSpace(out.String())).To(Equal("[]"))
	})

	It("emits yaml as a document stream, not a list", func() {
		var out bytes.Buffer
		Expect(generator.Render(&out, generator.OutputYAML, items, unusedTable)).To(Succeed())

		// Separator between documents, so two documents carry one separator.
		Expect(strings.Count(out.String(), "\n---\n")).To(Equal(1))
		Expect(strings.Split(out.String(), "\n---\n")).To(HaveLen(2))
		Expect(out.String()).NotTo(ContainSubstring("- name:"))
	})

	It("names the format it cannot render", func() {
		var out bytes.Buffer
		err := generator.Render(&out, generator.Output("toml"), items, unusedTable)
		Expect(err).To(MatchError(generator.ErrUnsupportedOutput))
		Expect(err.Error()).To(ContainSubstring(`"toml"`))
		Expect(out.String()).To(BeEmpty())
	})
})
