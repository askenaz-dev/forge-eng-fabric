{{/*
service-http flavor helpers. Consumed via `{{ include "service-http.<name>" . }}` in
each service chart's `templates/*.yaml`.
*/}}

{{- define "service-http.fullname" -}}
{{- .Values.fullnameOverride | default .Chart.Name -}}
{{- end -}}

{{- define "service-http.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/part-of: forge
app.kubernetes.io/managed-by: helm
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
forge.platform/flavor: service-http
{{- end -}}

{{- define "service-http.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
{{- end -}}
