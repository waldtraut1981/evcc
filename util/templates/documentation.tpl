product:
{{- if .ProductBrand }}
  brand: {{ .ProductBrand }}
{{- end }}
{{- if .ProductDescription }}
  description: {{ .ProductDescription }}
{{- end }}
{{- if .ProductGroup }}
  group: {{ .ProductGroup }}
{{- end }}
{{- if .Capabilities }}
capabilities: [{{ range .Capabilities -}}"{{ . }}"{{- end }}]
{{- end }}
{{- if .Requirements }}
requirements: [{{ range .Requirements -}}"{{ . }}"{{- end }}]
{{- end }}
{{- if .RequirementDescription }}
description: |
{{ .RequirementDescription | indent 2 }}
{{- end }}

render:
{{ if .Usages }}{{ range .Usages -}}
- usage: {{ . }}
  default: |
    type: template
    template: {{ $.Template }}
    usage: {{ . }}
    {{- range $.Params }}
    {{- if eq .Name "modbus" }}
    # Modbus Start
{{ $.Modbus | indent 4 -}}
    # Modbus End
    {{- else if ne .Advanced true }}
    {{ .Name }}:
    {{- if len .Value }} {{ .Value }} {{ end }}
    {{- if ne (len .Values) 0 }} 
    {{ range .Values }}
      - {{ . }}
    {{ end }}
    {{- end }}
    {{- if .Help.DE }} # {{ .Help.DE }} {{ end }}{{ if ne .Required true }} # Optional {{ end -}}
    {{- end -}}
    {{ end }}
{{- if $.AdvancedParams }}
  advanced: |
    type: template
    template: {{ $.Template }}
    usage: {{ . }}
    {{- range $.Params }}
    {{- if eq .Name "modbus" }}
    # Modbus Start
{{ $.Modbus | indent 4 -}}
    # Modbus End
    {{- else }}
    {{ .Name }}:
    {{- if len .Value }} {{ .Value }} {{ end }}
    {{- if ne (len .Values) 0 }} 
    {{ range .Values }}
      - {{ . }}
    {{ end }}
    {{- end }}
    {{- if .Help.DE }} # {{ .Help.DE }} {{ end }}{{ if ne .Required true }} # Optional {{ end -}}
    {{- end -}}
    {{ end }}
{{ end }}
{{ end }}{{ else -}}
- default: |
    type: template
    template: {{ $.Template }}
    {{- range $.Params }}
    {{- if eq .Name "modbus" }}
    # Modbus Start
{{ $.Modbus | indent 4 -}}
    # Modbus End
    {{- else if ne .Advanced true }}
    {{ .Name }}:
    {{- if len .Value }} {{ .Value }} {{ end }}
    {{- if ne (len .Values) 0 }} 
    {{ range .Values }}
      - {{ . }}
    {{ end }}
    {{- end }}
    {{- if .Help.DE }} # {{ .Help.DE }} {{ end }}{{ if ne .Required true }} # Optional {{ end -}}
    {{- end -}}
    {{ end }}
{{- if $.AdvancedParams }}
  advanced: |
    type: template
    template: {{ $.Template }}
    {{- range $.Params }}
    {{- if eq .Name "modbus" }}
    # Modbus Start
{{ $.Modbus | indent 4 -}}
    # Modbus End
    {{- else }}
    {{ .Name }}:
    {{- if len .Value }} {{ .Value }} {{ end }}
    {{- if ne (len .Values) 0 }} 
    {{ range .Values }}
      - {{ . }}
    {{ end }}
    {{- end }}
    {{- if .Help.DE }} # {{ .Help.DE }} {{ end }}{{ if ne .Required true }} # Optional {{ end -}}
    {{- end -}}
    {{ end }}
{{ end }}{{ end }}
