// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command karta-operator runs the OSS Karta operator. It reconciles Karta
// CRs by validating their spec and verifying that the referenced
// CustomResourceDefinition is present in the cluster.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/run-ai/karta/operator/pkg"
	"github.com/run-ai/karta/operator/pkg/version"
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(kartav1alpha1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// stripCRDSchemas projects each CRD down to only the fields the reconciler
// reads, so that unrelated fields are never retained in the cache.
func stripCRDSchemas(i any) (any, error) {
	crd, ok := i.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return i, nil
	}
	versions := make([]apiextensionsv1.CustomResourceDefinitionVersion, len(crd.Spec.Versions))
	for j, v := range crd.Spec.Versions {
		versions[j] = apiextensionsv1.CustomResourceDefinitionVersion{
			Name:   v.Name,
			Served: v.Served,
		}
	}
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: crd.ObjectMeta,
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: crd.Spec.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: crd.Spec.Names.Kind,
			},
			Versions: versions,
		},
	}, nil
}

var _ toolscache.TransformFunc = stripCRDSchemas

func run() error {
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		leaderElectionID     string
		printVersion         bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080",
		"The address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "karta-operator.run.ai",
		"Name of the resource used for leader election.")
	flag.BoolVar(&printVersion, "version", false,
		"Print the operator version and exit.")

	zapOpts := zap.Options{Development: false}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	if printVersion {
		fmt.Println(version.Version)
		return nil
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	logger := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&apiextensionsv1.CustomResourceDefinition{}: {
					Transform: stripCRDSchemas,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	if err = pkg.NewReconciler(mgr.GetClient(), mgr.GetEventRecorder(pkg.ControllerName)).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup karta reconciler: %w", err)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("register healthz: %w", err)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("register readyz: %w", err)
	}

	logger.Info("Starting Karta operator manager", "version", version.Version)
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("manager exited: %w", err)
	}
	return nil
}
