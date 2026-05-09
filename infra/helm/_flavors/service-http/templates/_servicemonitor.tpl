{{- define "service-http.servicemonitor" -}}
{{- if .Values.serviceMonitor.enabled -}}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ include "service-http.fullname" . }}
  labels:
    {{- include "service-http.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "service-http.selectorLabels" . | nindent 6 }}
  endpoints:
    - port: http
      path: {{ .Values.serviceMonitor.path }}
      interval: {{ .Values.serviceMonitor.interval }}
{{- end -}}
{{- end -}}
