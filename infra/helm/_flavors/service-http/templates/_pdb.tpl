{{- define "service-http.pdb" -}}
{{- if .Values.podDisruptionBudget.enabled -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "service-http.fullname" . }}
  labels:
    {{- include "service-http.labels" . | nindent 4 }}
spec:
  minAvailable: {{ .Values.podDisruptionBudget.minAvailable }}
  selector:
    matchLabels:
      {{- include "service-http.selectorLabels" . | nindent 6 }}
{{- end -}}
{{- end -}}
