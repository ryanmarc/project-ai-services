Day N:

{{- if ne .AGENT_PORT "" }}
{{- if eq .AGENT_STATUS "running" }}

- AI Agent (A2A protocol) is available at http://{{ .HOST_IP }}:{{ .AGENT_PORT }}.
{{- else }}

- AI Agent is unavailable. Please make sure '{{ .AppName }}--agent' pod is running.
{{- end }}
{{- end }}

{{- if eq .SUMMARIZE_API_STATUS "running" }}

- Summarize API is available at http://{{ .HOST_IP }}:{{ .SUMMARIZE_API_PORT }}. Use this endpoint for document summarization via programmatic access.
{{- else }}

- Summarize API is unavailable. Please make sure '{{ .AppName }}--summarize-api' pod is running.
{{- end }}