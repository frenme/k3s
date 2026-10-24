{{- define "service.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "service.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{ include "service.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
{{- end }}

{{- define "service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "service.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
