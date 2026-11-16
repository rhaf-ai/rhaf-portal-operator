{{- define "rhaf-portal-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "rhaf-portal-operator.fullname" -}}
{{- if contains (include "rhaf-portal-operator.name" .) .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "rhaf-portal-operator.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "rhaf-portal-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "rhaf-portal-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- required "serviceAccount.name is required when serviceAccount.create is false" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
