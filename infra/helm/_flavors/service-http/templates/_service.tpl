{{- define "service-http.service" -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "service-http.fullname" . }}
  labels:
    {{- include "service-http.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
    {{- include "service-http.selectorLabels" . | nindent 4 }}
{{- end -}}
