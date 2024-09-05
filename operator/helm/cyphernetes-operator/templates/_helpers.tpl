{{/*
Expand the name of the chart.
*/}}
{{- define "cyphernetes-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cyphernetes-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "cyphernetes-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cyphernetes-operator.labels" -}}
helm.sh/chart: {{ include "cyphernetes-operator.chart" . }}
{{ include "cyphernetes-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cyphernetes-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cyphernetes-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cyphernetes-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cyphernetes-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* Validate values */}}
{{- define "cyphernetes-operator.validateValues" -}}
{{- if not .Values.watchedKinds -}}
{{- fail "At least one kind must be specified in .Values.watchedKinds" -}}
{{- end -}}
{{- range .Values.extraPermissions -}}
{{- if not .kind -}}
{{- fail "Each item in extraPermissions must have a 'kind' field" -}}
{{- end -}}
{{- if not .verbs -}}
{{- fail "Each item in extraPermissions must have a 'verbs' field" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* Get API Group */}}
{{- define "cyphernetes-operator.getAPIGroup" -}}
{{- $parts := splitList "." . -}}
{{- if gt (len $parts) 1 -}}
{{- $group := rest $parts | join "." -}}
{{- $group -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end -}}

{{/* Get Resource */}}
{{- define "cyphernetes-operator.getResource" -}}
{{- $parts := splitList "." . -}}
{{- if gt (len $parts) 0 -}}
{{- $resource := first $parts -}}
{{- $resource -}}
{{- else -}}
{{- . -}}
{{- end -}}
{{- end -}}