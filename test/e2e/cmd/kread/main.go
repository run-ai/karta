// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command kread runs one CR through the Karta library against a Karta definition and prints what Karta
// reads: its matched statuses, phase, and conditions. It is a debugging aid for the e2e recorder, to show
// what Karta reports for a CR the recorder could not classify (a gap in the definition).
//
// Usage: go run ./cmd/kread <karta-definition.yaml> <cr.yaml|cr.json>
package main

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: kread <karta-definition.yaml> <cr.yaml|cr.json>")
		os.Exit(2)
	}

	karta := &kartav1alpha1.Karta{}
	must(yaml.Unmarshal(read(os.Args[1]), karta), "parse definition")
	if karta.GetName() == "" {
		karta.SetName("under-test")
	}

	var obj map[string]interface{}
	must(yaml.Unmarshal(read(os.Args[2]), &obj), "parse CR")
	cr := &unstructured.Unstructured{Object: obj}

	root, err := resource.NewComponentFactoryFromObject(karta, cr).GetRootComponent()
	must(err, "root component")
	status, err := root.GetStatus(context.Background())
	must(err, "status")

	if status == nil {
		fmt.Println("matchedStatuses: (nil status)")
		return
	}
	fmt.Printf("matchedStatuses: %v\n", status.MatchedStatuses)
	if status.Phase != nil {
		fmt.Printf("phase: %v\n", *status.Phase)
	}
	for _, c := range status.Conditions {
		fmt.Printf("condition: %+v\n", c)
	}
}

func read(path string) []byte {
	b, err := os.ReadFile(path)
	must(err, "read "+path)
	return b
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
		os.Exit(1)
	}
}
