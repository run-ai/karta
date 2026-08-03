<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Webhook Certificates

The Karta admission webhook is served over TLS, so the API server needs a serving
certificate it trusts. The chart provisions this in one of two modes, selected by
`webhook.cert.provisionMode`.

## Auto (default)

```yaml
webhook:
  enabled: true
  cert:
    provisionMode: auto
```

In `auto` mode the operator self-signs the serving certificate, writes it to the
`karta-operator-webhook-cert` Secret, patches the CA bundle onto the webhook
configurations, and rotates the certificate before it expires. No cert-manager
and no user action are required. This is the recommended default.

## Manual

```yaml
webhook:
  enabled: true
  cert:
    provisionMode: manual
```

In `manual` mode the operator does not touch certificates. It only reads the
serving certificate from the mounted Secret. The chart renders no cert-manager
objects, so any external provider can be used. Two knobs wire the provider in:

- `webhook.cert.annotations` are added to both webhook configurations. Use this to
  let a controller inject the CA bundle (for example cert-manager's cainjector).
- `webhook.cert.caBundle` is a base64-encoded CA bundle stamped directly onto both
  webhook configurations. Use this when no injector is available.

The operator mounts the Secret named `karta-operator-webhook-cert`, so the
externally provided Secret must use that name and contain `tls.crt` and `tls.key`.

### Required SANs

The API server reaches the webhook through its Service, so the certificate must be
valid for both of these names (replace `<namespace>` with the release namespace):

```
karta-operator-webhook.<namespace>.svc
karta-operator-webhook.<namespace>.svc.cluster.local
```

### Using cert-manager

Create an issuer and a Certificate that writes the serving Secret:

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: karta-selfsigned
  namespace: <namespace>
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: karta-operator-webhook-cert
  namespace: <namespace>
spec:
  secretName: karta-operator-webhook-cert
  dnsNames:
    - karta-operator-webhook.<namespace>.svc
    - karta-operator-webhook.<namespace>.svc.cluster.local
  issuerRef:
    kind: Issuer
    name: karta-selfsigned
```

The `selfSigned` issuer above signs the certificate directly. To use a corporate
CA or Vault instead, point `issuerRef` at your own `Issuer` or `ClusterIssuer`.

Then install the chart in `manual` mode and let cainjector patch the CA bundle:

```bash
helm upgrade -i karta charts/karta -n <namespace> \
  --set webhook.cert.provisionMode=manual \
  --set webhook.cert.annotations."cert-manager\.io/inject-ca-from"=<namespace>/karta-operator-webhook-cert
```

The annotation value is `<namespace>/<certificate-name>`, pointing at the
Certificate resource, not the Secret.

### Using a static CA bundle

If no injector is available, create the serving Secret yourself and pass the CA
bundle to the chart:

```bash
helm upgrade -i karta charts/karta -n <namespace> \
  --set webhook.cert.provisionMode=manual \
  --set webhook.cert.caBundle="$(base64 -w0 < ca.crt)"
```

The Secret named `karta-operator-webhook-cert` must exist and hold the serving
`tls.crt` and `tls.key`, and `caBundle` must be the base64-encoded CA that signed
`tls.crt`.

## Values reference

| Value | Default | Purpose |
| --- | --- | --- |
| `webhook.enabled` | `true` | Enable the admission webhook and all its resources. |
| `webhook.cert.provisionMode` | `auto` | `auto` (operator self-signs and rotates) or `manual` (external certs). |
| `webhook.cert.annotations` | `{}` | Manual mode. Annotations for the webhook configurations, for example `cert-manager.io/inject-ca-from`. |
| `webhook.cert.caBundle` | `""` | Manual mode. Base64-encoded CA bundle stamped onto the webhook configurations. |

The webhook uses `failurePolicy: Ignore`, so a missing or not-yet-ready
certificate never blocks Karta creation. The reconciler stamps the same GVK index
labels as a backstop, so the webhook is a pure accelerator.
