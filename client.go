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
	"github.com/tomblancdev/stromboli-go/generated/client/execution"
	"github.com/tomblancdev/stromboli-go/generated/client/system"
	"github.com/tomblancdev/stromboli-go/generated/models"
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
// Execution:
//   - [Client.Run]: Execute Claude synchronously
//   - [Client.RunAsync]: Execute Claude asynchronously (returns job ID)
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
// Execution Methods
// ----------------------------------------------------------------------------

// Run executes Claude synchronously and waits for the result.
//
// This method blocks until Claude completes execution or an error occurs.
// For long-running tasks, consider using [Client.RunAsync] instead.
//
// Basic usage:
//
//	result, err := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt: "Hello, Claude!",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Output)
//
// With configuration:
//
//	result, err := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt:  "Review this code for security issues",
//	    Workdir: "/workspace",
//	    Claude: &stromboli.ClaudeOptions{
//	        Model:        stromboli.ModelSonnet,
//	        MaxBudgetUSD: 5.0,
//	        AllowedTools: []string{"Read", "Glob", "Grep"},
//	    },
//	    Podman: &stromboli.PodmanOptions{
//	        Memory:  "2g",
//	        Timeout: "10m",
//	        Volumes: []string{"/home/user/project:/workspace:ro"},
//	    },
//	})
//
// Continuing a conversation:
//
//	// First request
//	result1, _ := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt: "Remember: my favorite color is blue",
//	})
//
//	// Continue the conversation
//	result2, _ := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt: "What's my favorite color?",
//	    Claude: &stromboli.ClaudeOptions{
//	        SessionID: result1.SessionID,
//	        Resume:    true,
//	    },
//	})
//
// The context can be used for cancellation:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
//	defer cancel()
//	result, err := client.Run(ctx, req)
func (c *Client) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	if req == nil {
		return nil, newError("BAD_REQUEST", "request is required", 400, nil)
	}
	if req.Prompt == "" {
		return nil, newError("BAD_REQUEST", "prompt is required", 400, nil)
	}

	// Convert to generated model
	genReq := c.toGeneratedRunRequest(req)

	// Create request parameters
	params := execution.NewPostRunParams()
	params.SetContext(ctx)
	params.SetTimeout(c.timeout)
	params.SetRequest(genReq)

	// Execute request
	resp, err := c.api.Execution.PostRun(params)
	if err != nil {
		return nil, c.handleError(err, "failed to execute Claude")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty run response", 0, nil)
	}

	return &RunResponse{
		ID:        payload.ID,
		Status:    payload.Status,
		Output:    payload.Output,
		Error:     payload.Error,
		SessionID: payload.SessionID,
	}, nil
}

// RunAsync starts Claude execution asynchronously and returns a job ID.
//
// Use this method for long-running tasks. Poll the job status with
// [Client.GetJob] or configure a webhook to be notified on completion.
//
// Basic usage:
//
//	job, err := client.RunAsync(ctx, &stromboli.RunRequest{
//	    Prompt: "Analyze this large codebase",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Job started: %s\n", job.JobID)
//
// With webhook notification:
//
//	job, err := client.RunAsync(ctx, &stromboli.RunRequest{
//	    Prompt:     "Review all files in the project",
//	    WebhookURL: "https://example.com/webhook",
//	})
//
// Polling for completion:
//
//	job, _ := client.RunAsync(ctx, req)
//
//	for {
//	    status, err := client.GetJob(ctx, job.JobID)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    switch status.Status {
//	    case "completed":
//	        fmt.Println(status.Output)
//	        return
//	    case "failed":
//	        log.Fatalf("Job failed: %s", status.Error)
//	    case "running":
//	        fmt.Println("Still running...")
//	        time.Sleep(2 * time.Second)
//	    }
//	}
func (c *Client) RunAsync(ctx context.Context, req *RunRequest) (*AsyncRunResponse, error) {
	if req == nil {
		return nil, newError("BAD_REQUEST", "request is required", 400, nil)
	}
	if req.Prompt == "" {
		return nil, newError("BAD_REQUEST", "prompt is required", 400, nil)
	}

	// Convert to generated model
	genReq := c.toGeneratedRunRequest(req)

	// Create request parameters
	params := execution.NewPostRunAsyncParams()
	params.SetContext(ctx)
	params.SetTimeout(c.timeout)
	params.SetRequest(genReq)

	// Execute request
	resp, err := c.api.Execution.PostRunAsync(params)
	if err != nil {
		return nil, c.handleError(err, "failed to start async execution")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty async run response", 0, nil)
	}

	return &AsyncRunResponse{
		JobID: payload.JobID,
	}, nil
}

// toGeneratedRunRequest converts our RunRequest to the generated model.
func (c *Client) toGeneratedRunRequest(req *RunRequest) *models.RunRequest {
	prompt := req.Prompt
	genReq := &models.RunRequest{
		Prompt:     &prompt,
		Workdir:    req.Workdir,
		WebhookURL: req.WebhookURL,
	}

	// Convert Claude options
	if req.Claude != nil {
		genReq.Claude.Model = req.Claude.Model
		genReq.Claude.SessionID = req.Claude.SessionID
		genReq.Claude.Resume = req.Claude.Resume
		genReq.Claude.MaxBudgetUsd = req.Claude.MaxBudgetUSD
		genReq.Claude.SystemPrompt = req.Claude.SystemPrompt
		genReq.Claude.AppendSystemPrompt = req.Claude.AppendSystemPrompt
		genReq.Claude.AllowedTools = req.Claude.AllowedTools
		genReq.Claude.DisallowedTools = req.Claude.DisallowedTools
		genReq.Claude.DangerouslySkipPermissions = req.Claude.DangerouslySkipPermissions
		genReq.Claude.PermissionMode = req.Claude.PermissionMode
		genReq.Claude.OutputFormat = req.Claude.OutputFormat
		genReq.Claude.Verbose = req.Claude.Verbose
		genReq.Claude.Debug = req.Claude.Debug
		genReq.Claude.Continue = req.Claude.Continue
		genReq.Claude.Agent = req.Claude.Agent
		genReq.Claude.FallbackModel = req.Claude.FallbackModel
	}

	// Convert Podman options
	if req.Podman != nil {
		genReq.Podman.Memory = req.Podman.Memory
		genReq.Podman.Timeout = req.Podman.Timeout
		genReq.Podman.Cpus = req.Podman.Cpus
		genReq.Podman.CPUShares = req.Podman.CPUShares
		genReq.Podman.Volumes = req.Podman.Volumes
		genReq.Podman.Image = req.Podman.Image
		genReq.Podman.SecretsEnv = req.Podman.SecretsEnv
	}

	return genReq
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
