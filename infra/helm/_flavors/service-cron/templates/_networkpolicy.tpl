{{- define "service-cron.networkpolicy" -}}
{{- if .Values.networkPolicy.enabled -}}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "service-cron.fullname" . }}
  labels:
    {{- include "service-cron.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "service-cron.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
  ingress: []
  egress:
    {{- if .Values.networkPolicy.egressTo }}
    {{- toYaml .Values.networkPolicy.egressTo | nindent 4 }}
    {{- else }}
    - {}
    {{- end }}
{{- end -}}
{{- end -}}
