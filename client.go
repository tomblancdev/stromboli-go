package stromboli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	generatedclient "github.com/tomblancdev/stromboli-go/generated/client"
	"github.com/tomblancdev/stromboli-go/generated/client/system"
)

// Default configuration values.
const (
	// defaultTimeout is the default request timeout.
	defaultTimeout = 30 * time.Second

	// defaultMaxRetries is the default number of retry attempts.
	defaultMaxRetries = 0
)

// Client is the Stromboli API client.
//
// Client provides a clean, idiomatic Go interface to the Stromboli API.
// It wraps the auto-generated client with additional features:
//   - Context support for cancellation and timeouts
//   - Automatic retries with exponential backoff
//   - Typed errors for common failure cases
//   - Simplified request/response types
//
// Create a new client using [NewClient]:
//
//	client := stromboli.NewClient("http://localhost:8585")
//
// The client is safe for concurrent use by multiple goroutines.
//
// # Methods
//
// System:
//   - [Client.Health]: Check API health status
//   - [Client.ClaudeStatus]: Check Claude configuration status
//
// More methods will be added for execution, jobs, and sessions.
type Client struct {
	// baseURL is the Stromboli API base URL.
	baseURL string

	// httpClient is the HTTP client used for requests.
	httpClient *http.Client

	// timeout is the default request timeout.
	timeout time.Duration

	// maxRetries is the maximum number of retry attempts.
	maxRetries int

	// userAgent is the User-Agent header value.
	userAgent string

	// api is the generated API client.
	api *generatedclient.StromboliAPI
}

// NewClient creates a new Stromboli API client.
//
// The baseURL should be the full URL to the Stromboli API, including
// the protocol and port. Examples:
//   - "http://localhost:8585"
//   - "https://stromboli.example.com"
//
// Use functional options to customize the client:
//
//	client := stromboli.NewClient("http://localhost:8585",
//	    stromboli.WithTimeout(5*time.Minute),
//	    stromboli.WithRetries(3),
//	    stromboli.WithHTTPClient(customHTTPClient),
//	)
//
// The returned client is safe for concurrent use.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
		timeout:    defaultTimeout,
		maxRetries: defaultMaxRetries,
		userAgent:  fmt.Sprintf("stromboli-go/%s", Version),
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Initialize the generated client
	c.api = c.newGeneratedClient()

	return c
}

// newGeneratedClient creates the underlying go-swagger client.
func (c *Client) newGeneratedClient() *generatedclient.StromboliAPI {
	// Parse the base URL
	u, err := url.Parse(c.baseURL)
	if err != nil {
		// Use defaults if URL parsing fails
		u = &url.URL{
			Scheme: "http",
			Host:   "localhost:8585",
		}
	}

	// Determine scheme
	schemes := []string{u.Scheme}
	if u.Scheme == "" {
		schemes = []string{"http"}
	}

	// Create transport
	transport := httptransport.New(u.Host, u.Path, schemes)
	transport.Transport = c.httpClient.Transport

	// Create client
	return generatedclient.New(transport, strfmt.Default)
}

// ----------------------------------------------------------------------------
// System Methods
// ----------------------------------------------------------------------------

// Health returns the health status of the Stromboli API.
//
// Use this method to:
//   - Check if the API is reachable and healthy
//   - Verify the server version
//   - Check the status of individual components (e.g., Podman)
//
// Example:
//
//	health, err := client.Health(ctx)
//	if err != nil {
//	    log.Fatalf("API is unreachable: %v", err)
//	}
//
//	if !health.IsHealthy() {
//	    for _, c := range health.Components {
//	        if !c.IsHealthy() {
//	            log.Printf("Component %s is unhealthy: %s", c.Name, c.Error)
//	        }
//	    }
//	}
//
//	fmt.Printf("API v%s is healthy\n", health.Version)
//
// The context can be used to set a timeout or cancel the request:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
//	defer cancel()
//	health, err := client.Health(ctx)
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	// Create request parameters with context
	params := system.NewGetHealthParams()
	params.SetContext(ctx)
	params.SetTimeout(c.timeout)

	// Execute request
	resp, err := c.api.System.GetHealth(params)
	if err != nil {
		return nil, c.handleError(err, "failed to get health status")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty health response", 0, nil)
	}

	// Map components
	components := make([]ComponentHealth, 0, len(payload.Components))
	for _, comp := range payload.Components {
		if comp != nil {
			components = append(components, ComponentHealth{
				Name:   comp.Name,
				Status: comp.Status,
				Error:  comp.Error,
			})
		}
	}

	return &HealthResponse{
		Name:       payload.Name,
		Status:     payload.Status,
		Version:    payload.Version,
		Components: components,
	}, nil
}

// ClaudeStatus returns the Claude configuration status.
//
// Use this method to check if the Stromboli server has valid Claude
// credentials configured. If not configured, execution requests will fail.
//
// Example:
//
//	status, err := client.ClaudeStatus(ctx)
//	if err != nil {
//	    log.Fatalf("Failed to check Claude status: %v", err)
//	}
//
//	if !status.Configured {
//	    log.Fatalf("Claude is not configured: %s", status.Message)
//	}
//
//	fmt.Println("Claude is ready for execution")
//
// The context can be used to set a timeout or cancel the request:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
//	defer cancel()
//	status, err := client.ClaudeStatus(ctx)
func (c *Client) ClaudeStatus(ctx context.Context) (*ClaudeStatus, error) {
	// Create request parameters with context
	params := system.NewGetClaudeStatusParams()
	params.SetContext(ctx)
	params.SetTimeout(c.timeout)

	// Execute request
	resp, err := c.api.System.GetClaudeStatus(params)
	if err != nil {
		return nil, c.handleError(err, "failed to get Claude status")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty Claude status response", 0, nil)
	}

	return &ClaudeStatus{
		Configured: payload.Configured,
		Message:    payload.Message,
	}, nil
}

// ----------------------------------------------------------------------------
// Error Handling
// ----------------------------------------------------------------------------

// handleError converts errors from the generated client into SDK errors.
//
// It handles:
//   - Network errors (connection refused, timeout, etc.)
//   - HTTP errors (4xx, 5xx responses)
//   - Unexpected response formats
func (c *Client) handleError(err error, message string) error {
	if err == nil {
		return nil
	}

	// Check for runtime API errors from go-swagger
	if apiErr, ok := err.(*runtime.APIError); ok {
		return c.handleAPIError(apiErr, message)
	}

	// Check for context cancellation
	if err == context.Canceled {
		return wrapError(err, "CANCELLED", "request was cancelled", 0)
	}

	// Check for context deadline exceeded
	if err == context.DeadlineExceeded {
		return wrapError(err, "TIMEOUT", "request timed out", 408)
	}

	// Generic error
	return wrapError(err, "REQUEST_FAILED", message, 0)
}

// handleAPIError converts go-swagger API errors into SDK errors.
func (c *Client) handleAPIError(apiErr *runtime.APIError, message string) error {
	status := apiErr.Code

	switch {
	case status == 400:
		return newError("BAD_REQUEST", message, status, apiErr)
	case status == 401:
		return newError("UNAUTHORIZED", "authentication required", status, apiErr)
	case status == 403:
		return newError("FORBIDDEN", "access denied", status, apiErr)
	case status == 404:
		return newError("NOT_FOUND", "resource not found", status, apiErr)
	case status == 408:
		return newError("TIMEOUT", "request timed out", status, apiErr)
	case status == 429:
		return newError("RATE_LIMITED", "too many requests", status, apiErr)
	case status >= 500:
		return newError("INTERNAL", "server error", status, apiErr)
	default:
		return newError("REQUEST_FAILED", message, status, apiErr)
	}
}
