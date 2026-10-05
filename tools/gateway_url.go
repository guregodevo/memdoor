package tools

import "fmt"

var gatewayBaseURL string

// SetGatewayBaseURL sets the base URL for all tool API calls to the gateway.
// Must be called during server startup before any tool execution.
func SetGatewayBaseURL(host string, port int) {
	if host == "" || port == 0 {
		panic("tools: gateway base URL requires non-empty host and non-zero port")
	}
	gatewayBaseURL = fmt.Sprintf("http://%s:%d", host, port)
}

func apiURL(path string) string {
	if gatewayBaseURL == "" {
		panic("tools: gateway base URL not set — call SetGatewayBaseURL before using tools")
	}
	return gatewayBaseURL + path
}
