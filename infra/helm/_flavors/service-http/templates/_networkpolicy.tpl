{{- define "service-http.networkpolicy" -}}
{{- if .Values.networkPolicy.enabled -}}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "service-http.fullname" . }}
  labels:
    {{- include "service-http.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "service-http.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    {{- if .Values.networkPolicy.ingressFrom }}
    - from:
        {{- toYaml .Values.networkPolicy.ingressFrom | nindent 8 }}
      ports:
        - port: {{ .Values.service.port }}
          protocol: TCP
    {{- else }}
    - from: []
      ports:
        - port: {{ .Values.service.port }}
          protocol: TCP
    {{- end }}
  egress:
    {{- if .Values.networkPolicy.egressTo }}
    {{- toYaml .Values.networkPolicy.egressTo | nindent 4 }}
    {{- else }}
    - {}
    {{- end }}
{{- end -}}
{{- end -}}
