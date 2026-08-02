// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	rotator "github.com/open-policy-agent/cert-controller/pkg/rotator"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const bootstrapTimeout = 2 * time.Minute

const (
	CertSourceSelfSigned  = "selfSigned"
	CertSourceCertManager = "certManager"
)

func ValidCertSource(s string) bool {
	return s == CertSourceSelfSigned || s == CertSourceCertManager
}

const (
	defaultOperatorNamespace = "karta-system"
	certCAName               = "karta-ca"
	certCAOrganization       = "karta"
)

type CertOptions struct {
	CertDir               string
	SecretName            string
	ServiceName           string
	MutatingWebhookName   string
	ValidatingWebhookName string
}

func OperatorNamespace() string {
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}
	return defaultOperatorNamespace
}

// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update

func newCertRotator(opts CertOptions, namespace, controllerName string, ready chan struct{}) *rotator.CertRotator {
	return &rotator.CertRotator{
		SecretKey:      types.NamespacedName{Namespace: namespace, Name: opts.SecretName},
		CertDir:        opts.CertDir,
		CAName:         certCAName,
		CAOrganization: certCAOrganization,
		DNSName:        fmt.Sprintf("%s.%s.svc", opts.ServiceName, namespace),
		IsReady:        ready,
		ControllerName: controllerName,
		Webhooks: []rotator.WebhookInfo{
			{Type: rotator.Mutating, Name: opts.MutatingWebhookName},
			{Type: rotator.Validating, Name: opts.ValidatingWebhookName},
		},
	}
}

func BootstrapCerts(ctx context.Context, kubeConfig *rest.Config, opts CertOptions, healthAddr string) error {
	ctx, cancel := context.WithTimeout(ctx, bootstrapTimeout)
	defer cancel()

	bootstrapMgr, err := ctrl.NewManager(kubeConfig, ctrl.Options{
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: healthAddr,
	})
	if err != nil {
		return fmt.Errorf("create bootstrap manager: %w", err)
	}
	if healthAddr != "" {
		if err := bootstrapMgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
			return fmt.Errorf("add bootstrap healthz: %w", err)
		}
	}

	ready := make(chan struct{})
	if err := rotator.AddRotator(bootstrapMgr, newCertRotator(opts, OperatorNamespace(), "karta-webhook-cert-bootstrap", ready)); err != nil {
		return fmt.Errorf("add bootstrap cert rotator: %w", err)
	}

	runCtx, stopMgr := context.WithCancel(ctx)
	defer stopMgr()

	startErr := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		if err := bootstrapMgr.Start(runCtx); err != nil {
			startErr <- err
		}
	}()

	select {
	case <-ready:
	case err := <-startErr:
		return fmt.Errorf("start bootstrap manager: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("wait for webhook cert: %w", ctx.Err())
	}

	stopMgr()
	select {
	case err := <-startErr:
		if err != nil {
			return fmt.Errorf("stop bootstrap manager: %w", err)
		}
	case <-stopped:
	case <-ctx.Done():
		return fmt.Errorf("wait for bootstrap shutdown: %w", ctx.Err())
	}
	return nil
}

func ManageCerts(mgr ctrl.Manager, opts CertOptions) error {
	ready := make(chan struct{})
	return rotator.AddRotator(mgr, newCertRotator(opts, OperatorNamespace(), "karta-webhook-cert", ready))
}
