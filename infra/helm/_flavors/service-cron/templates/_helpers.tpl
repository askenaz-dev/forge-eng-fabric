{{- define "service-cron.fullname" -}}
{{- .Values.fullnameOverride | default .Chart.Name -}}
{{- end -}}

{{- define "service-cron.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/part-of: forge
app.kubernetes.io/managed-by: helm
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
forge.platform/flavor: service-cron
{{- end -}}

{{- define "service-cron.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
{{- end -}}
