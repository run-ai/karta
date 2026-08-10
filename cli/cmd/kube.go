// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

func RESTConfig(rcg genericclioptions.RESTClientGetter) (*rest.Config, error) {
	cfg, err := rcg.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes config: %w", err)
	}
	return cfg, nil
}

func ResolvedNamespace(rcg genericclioptions.RESTClientGetter) (string, bool, error) {
	return rcg.ToRawKubeConfigLoader().Namespace()
}
