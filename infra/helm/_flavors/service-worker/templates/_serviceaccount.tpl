{{- define "service-worker.serviceaccount" -}}
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "service-worker.fullname" . }}
  labels:
    {{- include "service-worker.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- if .Values.serviceAccount.rbacRules }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "service-worker.fullname" . }}
  labels:
    {{- include "service-worker.labels" . | nindent 4 }}
rules:
  {{- toYaml .Values.serviceAccount.rbacRules | nindent 2 }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ include "service-worker.fullname" . }}
subjects:
  - kind: ServiceAccount
    name: {{ include "service-worker.fullname" . }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ include "service-worker.fullname" . }}
{{- end }}
{{- end -}}
{{- end -}}
