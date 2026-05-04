// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestListRendering(t *testing.T) {
	rows := []ListRow{
		{
			Namespace: "ml",
			Name:      "llama-finetune",
			Kind:      "PyTorchJob",
			Phases:    []string{"Running"},
			Components: []ComponentSummary{
				{Name: "master", CurrentReplicas: 1},
				{Name: "worker", CurrentReplicas: 4},
			},
			GPU: 33,
			Age: 2 * time.Hour,
		},
		{
			Namespace:  "ml",
			Name:       "preprocess",
			Kind:       "JobSet",
			Phases:     []string{"Completed"},
			Components: []ComponentSummary{{Name: "etl", CurrentReplicas: 3}},
			GPU:        0,
			Age:        45 * time.Minute,
		},
	}

	var buf bytes.Buffer
	if err := List(&buf, rows); err != nil {
		t.Fatalf("List render: %v", err)
	}
	got := buf.String()

	wants := []string{
		"NAMESPACE",
		"llama-finetune",
		"PyTorchJob",
		"master(1), worker(4)",
		"33",
		"2h",
		"preprocess",
		"etl(3)",
		"45m",
		"Completed",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("expected output to contain %q, got:\n%s", w, got)
		}
	}
}

func TestFormatAge(t *testing.T) {
	cases := map[time.Duration]string{
		0:                "-",
		30 * time.Second: "30s",
		5 * time.Minute:  "5m",
		3 * time.Hour:    "3h",
		48 * time.Hour:   "2d",
	}
	for d, want := range cases {
		if got := formatAge(d); got != want {
			t.Errorf("formatAge(%v): got %q want %q", d, got, want)
		}
	}
}
