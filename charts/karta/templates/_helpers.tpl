{{/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Base name for resources. Defaults to the chart name, overridable via nameOverride.
*/}}
{{- define "karta.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified resource name. Defaults to "<name>-operator" (e.g. "karta-operator"),
overridable wholesale via fullnameOverride. Truncated to 63 chars for the DNS label limit.
*/}}
{{- define "karta.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-operator" (include "karta.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Common labels stamped onto every resource.
*/}}
{{- define "karta.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "karta.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels. These must be stable across upgrades, so they intentionally
exclude version/chart labels (which change between releases).
*/}}
{{- define "karta.selectorLabels" -}}
app.kubernetes.io/name: {{ include "karta.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Name of the ServiceAccount to use. Honors serviceAccount.name when set,
otherwise derives from the fullname.
*/}}
{{- define "karta.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "karta.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}
