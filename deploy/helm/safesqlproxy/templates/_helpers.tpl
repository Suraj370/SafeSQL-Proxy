{{- define "safesqlproxy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "safesqlproxy.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "safesqlproxy.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "safesqlproxy.labels" -}}
app.kubernetes.io/name: {{ include "safesqlproxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end -}}

{{- define "safesqlproxy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "safesqlproxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
