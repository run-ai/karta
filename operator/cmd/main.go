// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command karta-operator runs the OSS Karta operator. It reconciles Karta
// CRs by validating their spec and verifying that the referenced
// CustomResourceDefinition is present in the cluster.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/run-ai/karta/operator/pkg"
	"github.com/run-ai/karta/operator/pkg/version"
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(kartav1alpha1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(scheme))
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		leaderElectionID     string
		printVersion         bool

		enableWebhook         bool
		webhookPort           int
		webhookCertDir        string
		webhookCertSource     string
		webhookCertSecret     string
		webhookServiceName    string
		mutatingWebhookName   string
		validatingWebhookName string
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

	flag.BoolVar(&enableWebhook, "enable-webhook", false,
		"Enable the Karta admission webhook.")
	flag.IntVar(&webhookPort, "webhook-port", 9443,
		"The port the webhook server binds to.")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"Directory the webhook serving cert is read from.")
	flag.StringVar(&webhookCertSource, "webhook-cert-source", pkg.CertSourceSelfSigned,
		"Webhook cert source: selfSigned (operator self-signs and rotates) or certManager.")
	flag.StringVar(&webhookCertSecret, "webhook-cert-secret", "karta-webhook-cert",
		"Name of the Secret holding the webhook serving cert in selfSigned mode.")
	flag.StringVar(&webhookServiceName, "webhook-service-name", "karta-operator-webhook",
		"Name of the webhook Service, used as the serving cert SAN.")
	flag.StringVar(&mutatingWebhookName, "mutating-webhook-name", "karta-operator-mutating",
		"Name of the MutatingWebhookConfiguration whose caBundle is patched in selfSigned mode.")
	flag.StringVar(&validatingWebhookName, "validating-webhook-name", "karta-operator-validating",
		"Name of the ValidatingWebhookConfiguration whose caBundle is patched in selfSigned mode.")

	zapOpts := zap.Options{Development: false}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	if printVersion {
		fmt.Println(version.Version)
		return nil
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	logger := ctrl.Log.WithName("setup")

	kubeConfig := ctrl.GetConfigOrDie()
	ctx := ctrl.SetupSignalHandler()

	if enableWebhook && !pkg.ValidCertSource(webhookCertSource) {
		return fmt.Errorf("invalid --webhook-cert-source %q (want selfSigned or certManager)", webhookCertSource)
	}

	certOpts := pkg.CertOptions{
		CertDir:               webhookCertDir,
		SecretName:            webhookCertSecret,
		ServiceName:           webhookServiceName,
		MutatingWebhookName:   mutatingWebhookName,
		ValidatingWebhookName: validatingWebhookName,
	}

	if enableWebhook && webhookCertSource == pkg.CertSourceSelfSigned {
		if err := pkg.BootstrapCerts(ctx, kubeConfig, certOpts, probeAddr); err != nil {
			return fmt.Errorf("bootstrap webhook certs: %w", err)
		}
	}

	options := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&apiextensionsv1.CustomResourceDefinition{}: {
					Transform: pkg.TrimCRDFields,
				},
			},
		},
	}
	if enableWebhook {
		options.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: webhookCertDir,
		})
	}

	mgr, err := ctrl.NewManager(kubeConfig, options)
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	if enableWebhook && webhookCertSource == pkg.CertSourceSelfSigned {
		if err := pkg.ManageCerts(mgr, certOpts); err != nil {
			return fmt.Errorf("setup webhook cert rotation: %w", err)
		}
	}

	if err = pkg.NewReconciler(mgr.GetClient(), mgr.GetEventRecorder(pkg.ControllerName)).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup karta reconciler: %w", err)
	}

	if enableWebhook {
		if err = pkg.SetupWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("setup karta webhook: %w", err)
		}
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("register healthz: %w", err)
	}
	readyzCheck := healthz.Ping
	if enableWebhook {
		readyzCheck = webhookReadyz(mgr)
	}
	if err = mgr.AddReadyzCheck("readyz", readyzCheck); err != nil {
		return fmt.Errorf("register readyz: %w", err)
	}

	logger.Info("Starting Karta operator manager", "version", version.Version)
	if err = mgr.Start(ctx); err != nil {
		return fmt.Errorf("manager exited: %w", err)
	}
	return nil
}

func webhookReadyz(mgr ctrl.Manager) healthz.Checker {
	return func(req *http.Request) error {
		return mgr.GetWebhookServer().StartedChecker()(req)
	}
}
