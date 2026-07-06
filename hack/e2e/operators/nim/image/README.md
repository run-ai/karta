<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Fictive CPU NIM image

A small stand-in for an NVIDIA NIM container, used by the Karta e2e cluster so the
k8s-nim-operator can drive a NIMService to Ready without a GPU or a real NGC token. It is a FastAPI server (see `nim_llm.py`) that answers the endpoints
the k8s-nim-operator probes, in particular `GET /v1/health/ready`, so the
operator drives the NIMService to `state=Ready`. It serves dummy responses only;
it loads no model and performs no inference.

It is a fictive stand-in authored for these e2e tests; it is not a real NIM,
serves no model, and requires no NGC credentials.

## What Karta uses

The operator only needs the readiness endpoint to mark the NIMService ready:

- HTTP on port 8000, gRPC on port 8001 (see `nim_service.proto`).
- `GET /v1/health/ready` and `GET /v1/health/live` for health probing.
- `GET /v1/models`, `POST /v1/chat/completions`, `GET /v1/metrics` for
  completeness; the Karta suite does not exercise these.

## How the suite builds it

`hack/e2e/up.sh` builds and loads the image into the kind cluster:

```sh
docker build -t nim-cpu:e2e hack/e2e/operators/nim/image
kind load docker-image nim-cpu:e2e --name karta-e2e
```

The NIMService smoke test (`hack/e2e/operators/nim/smoke.yaml`) references
`nim-cpu:e2e`, and `up.sh` creates a dummy `ngc-secret` (`NGC_API_KEY`) that the
operator requires but the image ignores.

## Run standalone

```sh
docker build -t nim-cpu:e2e hack/e2e/operators/nim/image
docker run -p 8000:8000 -p 8001:8001 nim-cpu:e2e
curl http://localhost:8000/v1/health/ready
```

`NIM_REQUEST_LATENCY` (float seconds, default 2.0) controls how long each request
takes, simulating processing time.

## Files

- `nim_llm.py` - the FastAPI + gRPC server.
- `nim_service.proto` - the gRPC service definition; Python stubs are generated
  at image build time.
- `Dockerfile` - `python:3.11-slim`, exposes 8000 and 8001.
- `requirements.txt` - Python dependencies.
