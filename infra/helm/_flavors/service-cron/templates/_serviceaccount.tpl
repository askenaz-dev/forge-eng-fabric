{{- define "service-cron.serviceaccount" -}}
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "service-cron.fullname" . }}
  labels:
    {{- include "service-cron.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- if .Values.serviceAccount.rbacRules }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "service-cron.fullname" . }}
  labels:
    {{- include "service-cron.labels" . | nindent 4 }}
rules:
  {{- toYaml .Values.serviceAccount.rbacRules | nindent 2 }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ include "service-cron.fullname" . }}
subjects:
  - kind: ServiceAccount
    name: {{ include "service-cron.fullname" . }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ include "service-cron.fullname" . }}
{{- end }}
{{- end -}}
{{- end -}}
