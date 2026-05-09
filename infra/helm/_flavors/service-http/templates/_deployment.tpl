{{- define "service-http.deployment" -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "service-http.fullname" . }}
  labels:
    {{- include "service-http.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "service-http.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "service-http.labels" . | nindent 8 }}
    spec:
      serviceAccountName: {{ include "service-http.fullname" . }}
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.service.port }}
          env:
            {{- range $name, $value := .Values.env }}
            - name: {{ $name }}
              value: {{ $value | quote }}
            {{- end }}
          {{- if .Values.probes.enabled }}
          readinessProbe:
            httpGet: { path: {{ .Values.probes.readinessPath | quote }}, port: http }
          livenessProbe:
            httpGet: { path: {{ .Values.probes.livenessPath | quote }}, port: http }
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
