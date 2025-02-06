{{/*
Expand the name of the chart.
*/}}
{{- define "k6-crds.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "k6-crds.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "k6-crds.labels" -}}
helm.sh/chart: {{ include "k6-crds.chart" . }}
{{ include "k6-crds.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: k6-operator
{{- end }}

{{/*
Selector labels
*/}}
{{- define "k6-crds.selectorLabels" -}}
app.kubernetes.io/name: {{ include "k6-crds.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "k6-crds.customLabels" -}}
  {{- if .Values.customLabels }}
    {{- with .Values.customLabels }}
      {{- toYaml . }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- define "k6-crds.customAnnotations" -}}
  {{- if .Values.customAnnotations }}
    {{- with .Values.customAnnotations }}
      {{- toYaml . }}
    {{- end }}
  {{- end }}
{{- end -}}
