{{- define "__APP__.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "__APP__.name" -}}
{{- if .Values.nameOverride -}}
{{- .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "__APP__.labels" -}}
{{ include "__APP__.selectorLabels" .}}
{{- if .Chart.Version }}
{{ printf "helm.sh/chart: %s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- if .Chart.AppVersion }}
{{ printf "app.kubernetes.io/version: '%s'" .Chart.AppVersion }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "__APP__.selectorLabels" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{ printf "app.kubernetes.io/name: %s" $name }}
{{ printf "app.kubernetes.io/instance: %s" .Release.Name }}
{{- end -}}
