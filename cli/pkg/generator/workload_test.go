// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator_test

import (
	"bytes"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/run-ai/karta/cli/pkg/generator"
	"github.com/run-ai/karta/cli/pkg/workload"
)

var views = []workload.View{{
	Name:      "preprocess",
	Namespace: "ml-team",
	Kind:      "JobSet",
	Phases:    []string{"Completed"},
	Origin:    "catalog",
}}

// cells splits a rendered table into its header and row fields. The tab writer
// pads with spaces and no cell contains one, so splitting on whitespace is
// enough, and comparing the fields pins the column set and its order. A
// substring check could not: "NAME" is a substring of "NAMESPACE".
func cells(out string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		rows = append(rows, strings.Fields(line))
	}
	return rows
}

var _ = Describe("RenderWorkloads", func() {
	DescribeTable("renders the table a format asks for",
		func(opts generator.Options, headers, row []string) {
			var out, errOut bytes.Buffer
			Expect(generator.RenderWorkloads(&out, &errOut, views, opts)).To(Succeed())

			Expect(cells(out.String())).To(Equal([][]string{headers, row}))
			Expect(errOut.Len()).To(BeZero())
		},
		Entry("the default columns",
			generator.Options{Output: generator.OutputTable},
			[]string{"NAME", "NAMESPACE", "PHASE", "AGE"},
			[]string{"preprocess", "ml-team", "Completed", "<unknown>"}),
		// Options{} has to stay usable, since table is the flag's default too.
		Entry("the zero value, as the default table",
			generator.Options{},
			[]string{"NAME", "NAMESPACE", "PHASE", "AGE"},
			[]string{"preprocess", "ml-team", "Completed", "<unknown>"}),
		Entry("wide, which adds ORIGIN",
			generator.Options{Output: generator.OutputWide},
			[]string{"NAME", "NAMESPACE", "PHASE", "AGE", "ORIGIN"},
			[]string{"preprocess", "ml-team", "Completed", "<unknown>", "catalog"}),
	)

	// A view resolved outside a live read carries no timestamp, which must not
	// saturate to the age of the zero time.
	It("reports an unset timestamp as unknown", func() {
		var out, errOut bytes.Buffer
		Expect(generator.RenderWorkloads(&out, &errOut,
			[]workload.View{{Name: "offline"}}, generator.Options{})).To(Succeed())

		Expect(out.String()).To(ContainSubstring("<unknown>"))
	})

	// The flag layer rejects an unknown format, so reaching the renderer with one
	// means a programmatic caller passed it; a table would misrepresent the ask.
	It("rejects a format that is set but unrecognized", func() {
		var out, errOut bytes.Buffer
		err := generator.RenderWorkloads(&out, &errOut, views, generator.Options{Output: "bogus"})

		Expect(errors.Is(err, generator.ErrUnsupportedOutput)).To(BeTrue())
		Expect(out.Len()).To(BeZero())
	})

	Describe("the empty result", func() {
		// The notice must not reach stdout, where it would corrupt a pipe.
		It("reports the namespace on stderr and exits cleanly", func() {
			var out, errOut bytes.Buffer
			Expect(generator.RenderWorkloads(&out, &errOut, nil,
				generator.Options{Output: generator.OutputTable, Namespace: "ml-team"})).To(Succeed())

			Expect(out.Len()).To(BeZero())
			Expect(errOut.String()).To(ContainSubstring("No workloads found in namespace ml-team."))
		})

		It("drops the namespace when every namespace was searched", func() {
			var out, errOut bytes.Buffer
			Expect(generator.RenderWorkloads(&out, &errOut, nil,
				generator.Options{Output: generator.OutputTable, AllNamespaces: true})).To(Succeed())

			Expect(errOut.String()).To(ContainSubstring("No workloads found in any namespace."))
		})

		// Machine output carries the result alone, so a consumer parsing stdout
		// never has to strip a human message.
		It("stays out of the machine formats", func() {
			var out, errOut bytes.Buffer
			Expect(generator.RenderWorkloads(&out, &errOut, nil,
				generator.Options{Output: generator.OutputJSON, Namespace: "ml-team"})).To(Succeed())

			Expect(strings.TrimSpace(out.String())).To(Equal("[]"))
			Expect(errOut.Len()).To(BeZero())
		})
	})
})
