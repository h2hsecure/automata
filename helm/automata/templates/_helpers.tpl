{{/*
Expand the name of the chart.
*/}}
{{- define "automata.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "automata.fullname" -}}
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
{{- define "automata.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "automata.labels" -}}
helm.sh/chart: {{ include "automata.chart" . }}
{{ include "automata.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "automata.selectorLabels" -}}
app.kubernetes.io/name: {{ include "automata.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Automata selector labels
*/}}
{{- define "automata.automataSelectorLabels" -}}
{{ include "automata.selectorLabels" . }}
app.kubernetes.io/component: automata
{{- end }}

{{/*
BFF selector labels
*/}}
{{- define "automata.bffSelectorLabels" -}}
{{ include "automata.selectorLabels" . }}
app.kubernetes.io/component: bff
{{- end }}

{{/*
Init container that blocks pod startup until PostgreSQL accepts connections.
Without it, a fresh install starts the app before the subchart's Postgres is
ready, and the app crash-loops (observed as RESTARTS on clean installs).
DB host/name/user/port mirror configmap.yaml so the wait targets the same
server the app connects to. pg_isready needs no credentials.
*/}}
{{- define "automata.waitForPostgresInitContainer" -}}
{{- $dbHost := ternary (printf "%s-postgresql" .Release.Name) (.Values.database.host | default "automata-postgresql") .Values.postgresql.enabled -}}
{{- $dbName := ternary (.Values.postgresql.auth.database | default "automata") (.Values.database.name | default "automata") .Values.postgresql.enabled -}}
{{- $dbUser := ternary (.Values.postgresql.auth.username | default "automata") (.Values.database.user | default "automata") .Values.postgresql.enabled -}}
{{- $dbPort := .Values.database.port | default "5432" -}}
- name: wait-for-postgres
  image: postgres:17-alpine
  command:
    - sh
    - -c
    - |
      until pg_isready -h {{ $dbHost }} -p {{ $dbPort }} -U {{ $dbUser }} -d {{ $dbName }} -q; do
        echo "waiting for postgres at {{ $dbHost }}:{{ $dbPort }}..."
        sleep 2
      done
{{- end }}
