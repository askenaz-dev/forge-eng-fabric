{{- define "service-worker.pdb" -}}
{{- if .Values.podDisruptionBudget.enabled -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "service-worker.fullname" . }}
  labels:
    {{- include "service-worker.labels" . | nindent 4 }}
spec:
  minAvailable: {{ .Values.podDisruptionBudget.minAvailable }}
  selector:
    matchLabels:
      {{- include "service-worker.selectorLabels" . | nindent 6 }}
{{- end -}}
{{- end -}}
