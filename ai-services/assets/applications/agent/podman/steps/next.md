- Use the AI Agent at http://{{ .HOST_IP }}:{{ .AGENT_PORT }} for A2A (Agent-to-Agent) protocol interactions.

- Use the Summarize API at http://{{ .HOST_IP }}:{{ .SUMMARIZE_API_PORT }} for document summarization.

- Run "ai-services application info {{ .AppName }} --runtime podman" to view service endpoints.