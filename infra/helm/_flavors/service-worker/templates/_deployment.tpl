{{- define "service-worker.deployment" -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "service-worker.fullname" . }}
  labels:
    {{- include "service-worker.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "service-worker.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "service-worker.labels" . | nindent 8 }}
    spec:
      serviceAccountName: {{ include "service-worker.fullname" . }}
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: metrics
              containerPort: {{ .Values.metricsPort }}
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
