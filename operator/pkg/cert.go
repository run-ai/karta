// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	rotator "github.com/open-policy-agent/cert-controller/pkg/rotator"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const bootstrapTimeout = 2 * time.Minute

const (
	// CertModeAuto: the operator self-signs and rotates the webhook cert.
	// CertModeManual: certs are provided externally (cert-manager, an admin, ...)
	// and the operator only reads them from the mounted Secret.
	CertModeAuto   = "auto"
	CertModeManual = "manual"
)

func ValidCertMode(s string) bool {
	return s == CertModeAuto || s == CertModeManual
}

const (
	certCAName         = "karta-ca"
	certCAOrganization = "karta"
)

type CertOptions struct {
	Namespace             string
	CertDir               string
	SecretName            string
	ServiceName           string
	MutatingWebhookName   string
	ValidatingWebhookName string
}

// ResolveNamespace returns the namespace the operator runs in. It prefers the
// explicit override (the chart passes --webhook-namespace from .Release.Namespace)
// and falls back to the projected service account namespace. It errors instead of
// guessing, so a wrong namespace can never silently break the webhook serving cert.
func ResolveNamespace(override string) (string, error) {
	if ns := strings.TrimSpace(override); ns != "" {
		return ns, nil
	}
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns, nil
		}
	}
	return "", errors.New("cannot determine operator namespace; set --webhook-namespace")
}

// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update

func newCertRotator(opts CertOptions, controllerName string, ready chan struct{}) *rotator.CertRotator {
	svcDNS := fmt.Sprintf("%s.%s.svc", opts.ServiceName, opts.Namespace)
	return &rotator.CertRotator{
		SecretKey:      types.NamespacedName{Namespace: opts.Namespace, Name: opts.SecretName},
		CertDir:        opts.CertDir,
		CAName:         certCAName,
		CAOrganization: certCAOrganization,
		DNSName:        svcDNS,
		ExtraDNSNames:  []string{svcDNS + ".cluster.local"},
		IsReady:        ready,
		// Only the leader rotates the shared cert Secret; other replicas receive
		// it through the mounted Secret. A manager without leader election is
		// treated as elected, so this still runs in single-replica and bootstrap.
		RequireLeaderElection: true,
		ControllerName:        controllerName,
		Webhooks: []rotator.WebhookInfo{
			{Type: rotator.Mutating, Name: opts.MutatingWebhookName},
			{Type: rotator.Validating, Name: opts.ValidatingWebhookName},
		},
	}
}

// BootstrapCerts blocks until the webhook serving cert exists on disk and its CA
// is patched onto the webhook configs, so the webhook server never starts before
// its cert is ready. It runs a throwaway manager whose only job is the cert
// rotator, then stops it. The manager binds no ports; the deployment's
// startupProbe covers this window so the liveness probe does not kill the pod.
func BootstrapCerts(ctx context.Context, kubeConfig *rest.Config, opts CertOptions) error {
	ctx, cancel := context.WithTimeout(ctx, bootstrapTimeout)
	defer cancel()

	mgr, err := ctrl.NewManager(kubeConfig, ctrl.Options{
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		return fmt.Errorf("create bootstrap manager: %w", err)
	}

	certReady := make(chan struct{})
	if err := rotator.AddRotator(mgr, newCertRotator(opts, "karta-webhook-cert-bootstrap", certReady)); err != nil {
		return fmt.Errorf("add bootstrap cert rotator: %w", err)
	}

	mgrCtx, stopMgr := context.WithCancel(ctx)
	defer stopMgr()
	mgrExited := make(chan error, 1)
	go func() { mgrExited <- mgr.Start(mgrCtx) }()

	select {
	case <-certReady:
	case err := <-mgrExited:
		if err == nil {
			err = ctx.Err()
		}
		return fmt.Errorf("bootstrap manager exited before cert was ready: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for webhook cert: %w", ctx.Err())
	}

	// Stop the throwaway manager and wait for it to release the health port
	// before the real manager binds it.
	stopMgr()
	if err := <-mgrExited; err != nil {
		return fmt.Errorf("stop bootstrap manager: %w", err)
	}
	return nil
}

func ManageCerts(mgr ctrl.Manager, opts CertOptions) error {
	ready := make(chan struct{})
	return rotator.AddRotator(mgr, newCertRotator(opts, "karta-webhook-cert", ready))
}
