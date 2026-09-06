{{- define "osint.name" -}}
{{ .Release.Name }}-baltic-osint-hub
{{- end }}

{{- define "osint.labels" -}}
app.kubernetes.io/name: baltic-osint-hub
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/* Image reference: digest wins over tag when set. */}}
{{- define "osint.image" -}}
{{- if .Values.image.digest -}}
{{ .Values.image.repository }}@{{ .Values.image.digest }}
{{- else -}}
{{ .Values.image.repository }}:{{ .Values.image.tag }}
{{- end -}}
{{- end }}

{{/*
Pull policy: explicit value wins; otherwise Always for the moving `latest`
tag and IfNotPresent for anything pinned (a sha tag or a digest is immutable,
so re-pulling it on every restart is wasted registry traffic).
*/}}
{{- define "osint.pullPolicy" -}}
{{- if .Values.image.pullPolicy -}}
{{ .Values.image.pullPolicy }}
{{- else if and (not .Values.image.digest) (eq .Values.image.tag "latest") -}}
Always
{{- else -}}
IfNotPresent
{{- end -}}
{{- end }}

{{/* Name of the chart-managed Postgres Secret. */}}
{{- define "osint.postgresSecret" -}}
{{ include "osint.name" . }}-postgres
{{- end }}

{{/*
Postgres password. Precedence: explicit value, then whatever the existing
Secret already holds (so an upgrade never rotates it), then a fresh random
one. `lookup` returns nothing under `helm template`, so a dry render always
shows a random value — that is expected.
*/}}
{{- define "osint.postgresPassword" -}}
{{- if .Values.postgres.password -}}
{{ .Values.postgres.password }}
{{- else -}}
{{- $existing := (lookup "v1" "Secret" .Release.Namespace (include "osint.postgresSecret" .)) -}}
{{- if and $existing $existing.data (index $existing.data "POSTGRES_PASSWORD") -}}
{{ index $existing.data "POSTGRES_PASSWORD" | b64dec }}
{{- else -}}
{{ randAlphaNum 32 }}
{{- end -}}
{{- end -}}
{{- end }}

{{/*
DATABASE_URL env for server and collector: from the chart-managed Postgres
Secret, else from the user's existingSecret.
*/}}
{{- define "osint.dbEnv" -}}
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
{{- if .Values.postgres.enabled }}
      name: {{ include "osint.postgresSecret" . }}
      key: DATABASE_URL
{{- else }}
      name: {{ .Values.existingSecret }}
      key: DATABASE_URL
{{- end }}
{{- end }}

{{/*
Hardened pod/container security for the Go workloads. The image is distroless
static:nonroot (uid 65532) and the binaries write nothing to disk, so the root
filesystem can be read-only.
*/}}
{{- define "osint.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 65532
runAsGroup: 65532
seccompProfile:
  type: RuntimeDefault
{{- end }}

{{- define "osint.containerSecurityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop: ["ALL"]
{{- end }}
