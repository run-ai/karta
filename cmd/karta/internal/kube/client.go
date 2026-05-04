// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package kube wires the standard kubectl-style configuration flags into
// the dynamic client and REST mapper that the karta CLI uses to read
// workload objects and pods. It deliberately wraps genericclioptions so
// every command sees the same kubeconfig / context / namespace plumbing.
package kube

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// Client bundles the kube clients the karta CLI needs.
type Client struct {
	config    *rest.Config
	dynamic   dynamic.Interface
	core      kubernetes.Interface
	mapper    *restmapper.DeferredDiscoveryRESTMapper
	namespace string
}

// NewClient resolves kubeconfig + context + namespace from standard kubectl flags
// and returns a Client ready to read workloads and pods.
func NewClient(flags *genericclioptions.ConfigFlags) (*Client, error) {
	cfg, err := flags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}

	disc, err := flags.ToDiscoveryClient()
	if err != nil {
		return nil, fmt.Errorf("discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(disc)

	ns, _, err := flags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, fmt.Errorf("resolve namespace: %w", err)
	}

	return &Client{
		config:    cfg,
		dynamic:   dyn,
		core:      core,
		mapper:    mapper,
		namespace: ns,
	}, nil
}

// Dynamic returns the dynamic client used to read arbitrary CRD objects.
func (c *Client) Dynamic() dynamic.Interface { return c.dynamic }

// Core returns the typed kubernetes client used to list pods.
func (c *Client) Core() kubernetes.Interface { return c.core }

// Mapper returns the REST mapper used to resolve GVK ↔ GVR.
func (c *Client) Mapper() *restmapper.DeferredDiscoveryRESTMapper { return c.mapper }

// Namespace returns the namespace resolved from --namespace or the current context.
func (c *Client) Namespace() string { return c.namespace }
