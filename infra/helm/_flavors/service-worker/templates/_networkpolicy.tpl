{{- define "service-worker.networkpolicy" -}}
{{- if .Values.networkPolicy.enabled -}}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "service-worker.fullname" . }}
  labels:
    {{- include "service-worker.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "service-worker.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: prometheus
      ports:
        - port: {{ .Values.metricsPort }}
          protocol: TCP
  egress:
    {{- if .Values.networkPolicy.egressTo }}
    {{- toYaml .Values.networkPolicy.egressTo | nindent 4 }}
    {{- else }}
    - {}
    {{- end }}
{{- end -}}
{{- end -}}
