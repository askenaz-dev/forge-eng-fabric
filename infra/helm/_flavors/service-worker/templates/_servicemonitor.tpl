{{- define "service-worker.servicemonitor" -}}
{{- if .Values.serviceMonitor.enabled -}}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ include "service-worker.fullname" . }}-metrics
  labels:
    {{- include "service-worker.labels" . | nindent 4 }}
spec:
  type: ClusterIP
  clusterIP: None
  ports:
    - port: {{ .Values.metricsPort }}
      targetPort: metrics
      protocol: TCP
      name: metrics
  selector:
    {{- include "service-worker.selectorLabels" . | nindent 4 }}
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ include "service-worker.fullname" . }}
  labels:
    {{- include "service-worker.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "service-worker.selectorLabels" . | nindent 6 }}
  endpoints:
    - port: metrics
      path: {{ .Values.serviceMonitor.path }}
      interval: {{ .Values.serviceMonitor.interval }}
{{- end -}}
{{- end -}}
