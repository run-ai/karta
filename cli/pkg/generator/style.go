// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Style holds the palette used by the renderers. A zero Style emits plain
// text - useful for tests, pipes, and NO_COLOR environments.
type Style struct {
	enabled bool
}

// AutoStyle returns a Style with colors enabled if w is a terminal and the
// NO_COLOR convention isn't set. Anything else returns a no-op Style.
func AutoStyle(w io.Writer) Style {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return Style{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return Style{}
	}
	if !term.IsTerminal(int(f.Fd())) {
		return Style{}
	}
	return Style{enabled: true}
}

// PlainStyle is a no-op style - every wrapper returns its input unchanged.
func PlainStyle() Style { return Style{} }

// ForceStyle returns a Style with colors enabled regardless of TTY status,
// for --color=always.
func ForceStyle() Style { return Style{enabled: true} }

const (
	resetSeq = "\x1b[0m"

	bold       = "\x1b[1m"
	dim        = "\x1b[2m"
	red        = "\x1b[31m"
	green      = "\x1b[32m"
	yellow     = "\x1b[33m"
	magenta    = "\x1b[35m"
	cyan       = "\x1b[36m"
	brightCyan = "\x1b[96m"
)

func (s Style) wrap(seq, text string) string {
	if !s.enabled || text == "" {
		return text
	}
	return seq + text + resetSeq
}

func (s Style) Bold(t string) string    { return s.wrap(bold, t) }
func (s Style) Dim(t string) string     { return s.wrap(dim, t) }
func (s Style) Red(t string) string     { return s.wrap(red, t) }
func (s Style) Green(t string) string   { return s.wrap(green, t) }
func (s Style) Yellow(t string) string  { return s.wrap(yellow, t) }
func (s Style) Magenta(t string) string { return s.wrap(magenta, t) }
func (s Style) Cyan(t string) string    { return s.wrap(cyan, t) }

// Header is the bold-bright-cyan "Kind/Name" lead-in.
func (s Style) Header(t string) string { return s.wrap(bold+brightCyan, t) }

// Phase colors a phase name by its semantics. Unrecognized phases render bold.
func (s Style) Phase(p string) string {
	switch p {
	case "Running", "Succeeded", "Completed":
		return s.Green(p)
	case "Initializing", "Pending", "Progressing":
		return s.Yellow(p)
	case "Failed", "Degraded":
		return s.Red(p)
	case "Undefined", "":
		return s.Dim(p)
	default:
		return s.Bold(p)
	}
}

// Phases joins and colors a phase list.
func (s Style) Phases(ps []string) string {
	if len(ps) == 0 {
		return s.Dim("-")
	}
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = s.Phase(p)
	}
	return strings.Join(parts, ",")
}

// Ratio colors a "got/want" pair: green when got >= want, yellow when
// partial, red when nothing is ready against a non-zero want.
func (s Style) Ratio(got, want int32, suffix string) string {
	body := suffix
	if body != "" {
		body = " " + body
	}
	text := formatRatio(got, want) + body
	switch {
	case want == 0:
		return s.Dim(text)
	case got >= want:
		return s.Green(text)
	case got == 0:
		return s.Red(text)
	default:
		return s.Yellow(text)
	}
}

func formatRatio(a, b int32) string {
	return itoa(int(a)) + "/" + itoa(int(b))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
