{{- define "service-worker.fullname" -}}
{{- .Values.fullnameOverride | default .Chart.Name -}}
{{- end -}}

{{- define "service-worker.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/part-of: forge
app.kubernetes.io/managed-by: helm
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
forge.platform/flavor: service-worker
{{- end -}}

{{- define "service-worker.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
{{- end -}}
