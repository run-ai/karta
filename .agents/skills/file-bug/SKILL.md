---
name: file-bug
description: >-
  File a well-structured GitHub bug report against run-ai/karta using
  context from the current conversation. Auto-collects environment info
  (Karta CRD version, Go version, K8s version, OS) and presents a
  template before submission. Use when reporting a Karta bug, error, or
  unexpected behavior.
user-invocable: true
---

# File a Karta Bug Report

Use the current conversation context to file a structured bug report against `run-ai/karta` via the `gh` CLI.

## Instructions

### 1. Gather context from the conversation

Review what the user has been working on, the problem encountered, error messages, logs, stack traces, and reproduction steps already discussed. If a critical detail is missing, ask once briefly. Prefer inferring from conversation context over asking.

### 2. Collect environment info

Auto-detect what is available. Mark unknowns as "N/A".

```bash
# OS and architecture
uname -s -m

# Go version (if a developer build)
go version 2>/dev/null

# K8s server version (if connected to a cluster)
kubectl version --output=json 2>/dev/null | jq -r '.serverVersion.gitVersion'

# Karta CRD installed version
kubectl get crd kartas.optimization.nvidia.com -o jsonpath='{.metadata.annotations.helm\.sh/chart}' 2>/dev/null

# Karta Helm release (if installed via Helm)
helm list -A -o json 2>/dev/null | jq -r '.[] | select(.chart | startswith("karta")) | "\(.chart) (\(.namespace))"'

# Karta git ref (if working from a checkout)
cd ~/Karta/karta 2>/dev/null && git rev-parse --short HEAD
```

### 3. Identify the affected component

Karta has several distinct surfaces. Match the error to one before drafting:

| Symptom | Likely component |
|---------|------------------|
| Karta CR rejected at apply / validation error | CRD schema (`pkg/api/optimization/v1alpha1/`) |
| Wrong replica count, missing pods in tree | Resource extraction (`pkg/resource/`) |
| Status not updating / stuck phase | Status mapping in the example file or `pkg/resource/` |
| JQ path returns null or wrong shape | JQ engine (`pkg/jq/`) |
| Gang scheduling wrong | Instructions (`pkg/instructions/`) |
| Helm install failure | Chart (`charts/karta/`) |
| CLI output broken / format issue | CLI (`cmd/`) |

If the symptom does not match any of these, default to "unknown" and explain in the description.

### 4. Draft the issue

Present this template to the user for review before filing:

```
**Describe the Bug**
<clear, concise description in one paragraph>

**Steps to Reproduce**
1. ...
2. ...
3. ...
<!-- Include the workload type, the Karta example used, and the manifest if relevant -->

**Expected Behavior**
<what should have happened>

**Actual Behavior**
<what actually happened>

<error messages, logs, stack traces — code-fenced>

**Affected Component**
<one of: CRD, resource extraction, status mapping, JQ engine, instructions, chart, CLI, unknown>

**Environment**
- OS: <e.g. Linux x86_64>
- Karta version: <chart version or git ref>
- Kubernetes version: <e.g. v1.30.2>
- Go version: <if running developer build, else N/A>
- Install method: <Helm release / kubectl apply / built from source>

**Additional Context**
<anything else: a workaround that helped, related issues, dashboards>
```

### 5. Search for duplicates

Before filing, run a quick `gh` search:

```bash
gh issue list --repo run-ai/karta --search "<short keyword from the symptom>" --state all --limit 10
```

If a likely duplicate exists, ask the user whether to add a comment on the existing issue instead of filing a new one.

### 6. File the issue

```bash
gh issue create \
  --repo run-ai/karta \
  --title "<concise bug title>" \
  --body "<filled template>" \
  --label bug
```

Return the issue URL to the user.

## Notes

- Do not paste internal Confluence/Jira links or customer manifests into the issue body. Sanitize logs and manifests before submission.
- If the user is reporting a security issue rather than a functional bug, redirect to `SECURITY.md` instead of filing a public issue.
- The `bug` label may not exist yet in the repo. If `gh` rejects it, drop the `--label` flag and add the label via the UI after creation.
