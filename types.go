package stromboli

// HealthResponse represents the health status of the Stromboli API.
//
// Use [Client.Health] to retrieve the current health status:
//
//	health, err := client.Health(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Status: %s, Version: %s\n", health.Status, health.Version)
type HealthResponse struct {
	// Name is the service name, typically "stromboli".
	Name string `json:"name"`

	// Status indicates the overall health status.
	// Values: "ok" (healthy) or "error" (unhealthy).
	Status string `json:"status"`

	// Version is the Stromboli server version.
	// Example: "0.3.0-alpha".
	Version string `json:"version"`

	// Components lists the health status of individual components.
	// Check this to identify which component is failing when Status is "error".
	Components []ComponentHealth `json:"components"`
}

// IsHealthy returns true if the overall status is "ok".
//
// Example:
//
//	health, _ := client.Health(ctx)
//	if !health.IsHealthy() {
//	    log.Println("API is unhealthy!")
//	}
func (h *HealthResponse) IsHealthy() bool {
	return h.Status == "ok"
}

// ComponentHealth represents the health status of an individual component.
//
// Stromboli checks the following components:
//   - "podman": Container runtime availability
//   - Additional components may be added in future versions
type ComponentHealth struct {
	// Name is the component identifier.
	// Example: "podman".
	Name string `json:"name"`

	// Status indicates the component health.
	// Values: "ok" (healthy) or "error" (unhealthy).
	Status string `json:"status"`

	// Error contains the error message when Status is "error".
	// Empty when Status is "ok".
	Error string `json:"error,omitempty"`
}

// IsHealthy returns true if the component status is "ok".
func (c *ComponentHealth) IsHealthy() bool {
	return c.Status == "ok"
}

// ClaudeStatus represents the Claude configuration status.
//
// Use [Client.ClaudeStatus] to check if Claude credentials are configured:
//
//	status, err := client.ClaudeStatus(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if status.Configured {
//	    fmt.Println("Claude is ready!")
//	} else {
//	    fmt.Printf("Claude not configured: %s\n", status.Message)
//	}
type ClaudeStatus struct {
	// Configured indicates whether Claude credentials are set up.
	// When false, execution requests will fail with an authentication error.
	Configured bool `json:"configured"`

	// Message provides additional context about the configuration status.
	// When Configured is true: "Claude is configured"
	// When Configured is false: explains what is missing
	Message string `json:"message"`
}

// HealthStatus constants for convenience.
const (
	// StatusOK indicates the service or component is healthy.
	StatusOK = "ok"

	// StatusError indicates the service or component has an error.
	StatusError = "error"
)
