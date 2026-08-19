// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/run-ai/karta/exporter/pkg/collector"
	"github.com/run-ai/karta/exporter/pkg/controller"
	"github.com/run-ai/karta/exporter/pkg/owner"
	"github.com/run-ai/karta/exporter/pkg/registry"
	"github.com/run-ai/karta/exporter/pkg/server"
	"github.com/run-ai/karta/exporter/pkg/store"
)

func main() {
	var (
		metricsAddr  string
		kubeconfig   string
		fullPodCache bool
		resync       time.Duration
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address to serve /metrics, /healthz, and /readyz on.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig; empty means in-cluster configuration.")
	flag.BoolVar(&fullPodCache, "full-pod-cache", false, "Cache full pod objects instead of the trimmed shape. Needed only for custom Kartas whose pod selectors read fields outside metadata, spec.nodeName, and status.phase.")
	flag.DurationVar(&resync, "resync-period", 10*time.Hour, "Informer resync period.")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(metricsAddr, kubeconfig, fullPodCache, resync, logger); err != nil {
		logger.Error("exporter failed", "error", err)
		os.Exit(1)
	}
}

func run(metricsAddr, kubeconfig string, fullPodCache bool, resync time.Duration, logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	restConfig, err := buildRESTConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build rest config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}
	metadataClient, err := metadata.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create metadata client: %w", err)
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	kartaRegistry := registry.New()
	ownerIndex := owner.New()
	metricsStore := store.New()

	prometheusRegistry := prometheus.NewRegistry()
	prometheusRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	exporterController := controller.New(
		dynamicClient, metadataClient, kubeClient, mapper,
		kartaRegistry, ownerIndex, metricsStore,
		prometheusRegistry, logger,
		controller.Options{FullPodCache: fullPodCache, Resync: resync},
	)

	prometheusRegistry.MustRegister(collector.New(metricsStore, collector.Options{
		PendingPods: ownerIndex.PendingCount,
		KartaCounts: func() (int, int, int) {
			stats := kartaRegistry.Stats()
			return stats.Valid, stats.Invalid, stats.Shadowed
		},
	}))

	metricsServer := server.New(metricsAddr, prometheusRegistry, exporterController.Ready)

	errCh := make(chan error, 2)
	go func() { errCh <- exporterController.Run(ctx) }()
	go func() { errCh <- metricsServer.Run(ctx) }()

	logger.Info("karta exporter started", "metricsAddr", metricsAddr, "fullPodCache", fullPodCache)

	select {
	case err := <-errCh:
		cancel()
		return err
	case <-ctx.Done():
		return <-errCh
	}
}

func buildRESTConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
}
