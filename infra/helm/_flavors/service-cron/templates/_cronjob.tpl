{{- define "service-cron.cronjob" -}}
apiVersion: batch/v1
kind: CronJob
metadata:
  name: {{ include "service-cron.fullname" . }}
  labels:
    {{- include "service-cron.labels" . | nindent 4 }}
spec:
  schedule: {{ .Values.schedule | quote }}
  concurrencyPolicy: {{ .Values.concurrencyPolicy }}
  successfulJobsHistoryLimit: {{ .Values.successfulJobsHistoryLimit }}
  failedJobsHistoryLimit: {{ .Values.failedJobsHistoryLimit }}
  jobTemplate:
    spec:
      backoffLimit: {{ .Values.backoffLimit }}
      template:
        metadata:
          labels:
            {{- include "service-cron.labels" . | nindent 12 }}
        spec:
          serviceAccountName: {{ include "service-cron.fullname" . }}
          restartPolicy: {{ .Values.restartPolicy }}
          containers:
            - name: {{ .Chart.Name }}
              image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
              imagePullPolicy: {{ .Values.image.pullPolicy }}
              env:
                {{- range $name, $value := .Values.env }}
                - name: {{ $name }}
                  value: {{ $value | quote }}
                {{- end }}
              resources:
                requests:
                  cpu: {{ .Values.resources.requests.cpu }}
                  memory: {{ .Values.resources.requests.memory }}
                limits:
                  cpu: {{ .Values.resources.limits.cpu }}
                  memory: {{ .Values.resources.limits.memory }}
              securityContext:
                allowPrivilegeEscalation: false
                readOnlyRootFilesystem: false
                runAsNonRoot: true
                runAsUser: 1000
                capabilities:
                  drop: ["ALL"]
{{- end -}}
