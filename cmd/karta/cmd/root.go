// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type rootOptions struct {
	configFlags *genericclioptions.ConfigFlags
}

func NewRootCmd() *cobra.Command {
	opts := &rootOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
	}

	root := &cobra.Command{
		Use:   "karta",
		Short: "Karta CLI — workload-aware visibility for any Kubernetes AI workload",
		Long: `Karta is a CLI that reads Karta workload definitions and renders a unified view
of any Kubernetes AI workload — components, roles, scaling, status, GPU allocation —
across PyTorchJob, RayCluster, JobSet, KServe, and any custom CRD with a Karta definition.

Same output shape regardless of the underlying CRD.`,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	opts.configFlags.AddFlags(root.PersistentFlags())

	root.AddCommand(newWorkloadCmd(opts))

	return root
}
