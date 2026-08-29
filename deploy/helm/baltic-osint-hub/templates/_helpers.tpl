{{- define "osint.name" -}}
{{ .Release.Name }}-baltic-osint-hub
{{- end }}

{{- define "osint.labels" -}}
app.kubernetes.io/name: baltic-osint-hub
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "osint.databaseURL" -}}
{{- if .Values.postgres.enabled -}}
postgres://{{ .Values.postgres.user }}:{{ .Values.postgres.password }}@{{ include "osint.name" . }}-postgres:5432/{{ .Values.postgres.database }}
{{- end -}}
{{- end }}

{{/* DATABASE_URL env: from chart-managed postgres, else from the secret. */}}
{{- define "osint.dbEnv" -}}
{{- if .Values.postgres.enabled }}
- name: DATABASE_URL
  value: {{ include "osint.databaseURL" . | quote }}
{{- else }}
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: {{ .Values.existingSecret }}
      key: DATABASE_URL
{{- end }}
{{- end }}
