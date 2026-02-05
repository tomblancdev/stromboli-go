package stromboli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	generatedclient "github.com/tomblancdev/stromboli-go/generated/client"
	"github.com/tomblancdev/stromboli-go/generated/client/auth"
	"github.com/tomblancdev/stromboli-go/generated/client/execution"
	"github.com/tomblancdev/stromboli-go/generated/client/images"
	"github.com/tomblancdev/stromboli-go/generated/client/jobs"
	"github.com/tomblancdev/stromboli-go/generated/client/secrets"
	"github.com/tomblancdev/stromboli-go/generated/client/sessions"
	"github.com/tomblancdev/stromboli-go/generated/client/system"
	"github.com/tomblancdev/stromboli-go/generated/models"
)

// Default configuration values.
const (
	// defaultTimeout is the default request timeout.
	defaultTimeout = 30 * time.Second

	// maxPromptSize limits the maximum prompt size to prevent memory exhaustion.
	// 1MB chosen based on Claude's typical context window (~200k tokens ≈ 800KB text)
	// with headroom for encoding overhead.
	maxPromptSize = 1 * 1024 * 1024 // 1MB

	// maxSystemPromptSize limits the maximum system prompt size.
	// System prompts are typically much shorter than user prompts.
	// 256KB allows for detailed instructions while maintaining safety.
	maxSystemPromptSize = 256 * 1024 // 256KB

	// maxJSONSchemaSize limits the maximum JSON schema size.
	// Most schemas are small (<10KB), but complex nested schemas can be larger.
	// 64KB accommodates all reasonable use cases.
	maxJSONSchemaSize = 64 * 1024 // 64KB
)

var (
	// defaultTransportOnce ensures we only clone DefaultTransport once.
	// This prevents potential races if DefaultTransport is modified concurrently.
	defaultTransportOnce sync.Once
	defaultTransportCopy *http.Transport
)

// getDefaultTransport returns a cached transport for client isolation.
// This is safe to call from multiple goroutines.
func getDefaultTransport() *http.Transport {
	defaultTransportOnce.Do(func() {
		if t, ok := http.DefaultTransport.(*http.Transport); ok {
			defaultTransportCopy = t.Clone()
		} else {
			// http.DefaultTransport was replaced with a custom implementation.
			// Create a fresh transport to ensure client isolation rather than
			// sharing the custom transport across all clients.
			getLogger().Printf("stromboli: WARNING: http.DefaultTransport is not *http.Transport, creating isolated transport")
			defaultTransportCopy = &http.Transport{
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}
		}
	})
	return defaultTransportCopy
}

// Client is the Stromboli API client.
//
// Client provides a clean, idiomatic Go interface to the Stromboli API.
// It wraps the auto-generated client with additional features:
//   - Context support for cancellation and timeouts
//   - Typed errors for common failure cases
//   - Simplified request/response types
//
// Create a new client using [NewClient]:
//
//	client, err := stromboli.NewClient("http://localhost:8585")
//	if err != nil {
//	    log.Fatal(err)
//	}
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
//
// Auth:
//   - [Client.GetToken]: Obtain JWT tokens
//   - [Client.RefreshToken]: Refresh access token
//   - [Client.ValidateToken]: Validate current token
//   - [Client.Logout]: Invalidate current token
//
// Secrets:
//   - [Client.ListSecrets]: List available Podman secrets
type Client struct {
	// baseURL is the Stromboli API base URL.
	baseURL string

	// httpClient is the HTTP client used for requests.
	httpClient *http.Client

	// timeout is the default request timeout.
	timeout time.Duration

	// streamTimeout is the default timeout for streaming requests.
	// If set and no context deadline exists, this timeout is applied.
	streamTimeout time.Duration

	// userAgent is the User-Agent header value.
	userAgent string

	// mu protects token for concurrent access.
	mu sync.RWMutex

	// token is the Bearer token for authenticated requests.
	token string

	// api is the generated API client.
	api *generatedclient.StromboliAPI

	// requestHook is called before each HTTP request (optional).
	requestHook RequestHook

	// responseHook is called after each HTTP response (optional).
	responseHook ResponseHook
}

// NewClient creates a new Stromboli API client.
//
// The baseURL should be the full URL to the Stromboli API, including
// the protocol and port. Examples:
//   - "http://localhost:8585"
//   - "https://stromboli.example.com"
//
// Returns an error if the URL is invalid or malformed.
//
// Use functional options to customize the client:
//
//	client, err := stromboli.NewClient("http://localhost:8585",
//	    stromboli.WithTimeout(5*time.Minute),
//	    stromboli.WithHTTPClient(customHTTPClient),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// The returned client is safe for concurrent use.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	// Validate URL upfront
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("stromboli: invalid base URL: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("stromboli: base URL must include host")
	}
	// Validate scheme (only http and https are supported)
	if u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("stromboli: unsupported URL scheme %q (use http or https)", u.Scheme)
	}

	c := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		timeout:    defaultTimeout,
		userAgent:  fmt.Sprintf("stromboli-go/%s", Version),
	}

	// Clone the cached transport to give this client its own connection pool.
	// This ensures clients don't interfere with each other's connections.
	// getDefaultTransport() uses sync.Once to ensure thread-safe initialization.
	if t := getDefaultTransport(); t != nil {
		c.httpClient.Transport = t.Clone()
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Initialize the generated client
	c.api = c.newGeneratedClient()

	return c, nil
}

// userAgentTransport wraps http.RoundTripper to add User-Agent header and invoke hooks.
type userAgentTransport struct {
	base         http.RoundTripper
	userAgent    string
	requestHook  RequestHook
	responseHook ResponseHook
}

// RoundTrip implements http.RoundTripper.
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.userAgent)

	// Call request hook unconditionally - request is always valid at this point.
	if t.requestHook != nil {
		t.requestHook(req)
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)

	// Call response hook only if we have a response.
	// On network errors, resp may be nil, so we skip the hook.
	// This asymmetry is intentional: request hooks fire for all requests,
	// response hooks fire only for successful network round-trips.
	if t.responseHook != nil && resp != nil {
		t.responseHook(resp)
	}

	return resp, err
}

// newGeneratedClient creates the underlying go-swagger client.
//
// NOTE: Request and response hooks are captured at client creation time.
// Changing hooks after client creation has no effect on the generated API client.
// To use different hooks, create a new client.
func (c *Client) newGeneratedClient() *generatedclient.StromboliAPI {
	// URL already validated in NewClient
	u, _ := url.Parse(c.baseURL)

	// Determine scheme
	schemes := []string{u.Scheme}
	if u.Scheme == "" {
		schemes = []string{"http"}
	}

	// Create transport with user agent and hooks
	transport := httptransport.New(u.Host, u.Path, schemes)
	transport.Transport = &userAgentTransport{
		base:         c.httpClient.Transport,
		userAgent:    c.userAgent,
		requestHook:  c.requestHook,
		responseHook: c.responseHook,
	}

	// Create client
	return generatedclient.New(transport, strfmt.Default)
}

// effectiveTimeout returns the shorter of the client timeout and context deadline.
// This ensures the documented behavior where the effective timeout is the minimum
// of the client's configured timeout and the context's deadline.
func (c *Client) effectiveTimeout(ctx context.Context) time.Duration {
	timeout := c.timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			// Deadline already passed - return 0 to let the request
			// fail immediately with context deadline exceeded error.
			return 0
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	return timeout
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
	params.SetTimeout(c.effectiveTimeout(ctx))

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

	// Map components.
	// Note: len(nil) returns 0 in Go, so this is safe even if Components is nil.
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
	params.SetTimeout(c.effectiveTimeout(ctx))

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
// # Timeout Behavior
//
// The effective request timeout is determined by the shorter of:
//   - The client's configured timeout (via [WithTimeout])
//   - The context's deadline (if set via [context.WithTimeout])
//
// For long-running tasks, either increase the client timeout or use
// [Client.RunAsync] instead.
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

	// Validate request size limits
	if err := validateRequestSize(req); err != nil {
		return nil, err
	}

	// Validate JSON schema if provided
	if req.Claude != nil && req.Claude.JSONSchema != "" {
		if err := validateJSONSchema(req.Claude.JSONSchema); err != nil {
			return nil, newError("BAD_REQUEST", fmt.Sprintf("invalid JSON schema: %v", err), 400, nil)
		}
	}

	// Validate Resume requires SessionID
	if req.Claude != nil && req.Claude.Resume && req.Claude.SessionID == "" {
		return nil, newError("BAD_REQUEST", "session_id is required when resume is true", 400, nil)
	}

	// Convert to generated model
	genReq := toGeneratedRunRequest(req)

	// Create request parameters
	params := execution.NewPostRunParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
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

	// Validate request size limits
	if err := validateRequestSize(req); err != nil {
		return nil, err
	}

	// Validate JSON schema if provided
	if req.Claude != nil && req.Claude.JSONSchema != "" {
		if err := validateJSONSchema(req.Claude.JSONSchema); err != nil {
			return nil, newError("BAD_REQUEST", fmt.Sprintf("invalid JSON schema: %v", err), 400, nil)
		}
	}

	// Validate Resume requires SessionID
	if req.Claude != nil && req.Claude.Resume && req.Claude.SessionID == "" {
		return nil, newError("BAD_REQUEST", "session_id is required when resume is true", 400, nil)
	}

	// Convert to generated model
	genReq := toGeneratedRunRequest(req)

	// Create request parameters
	params := execution.NewPostRunAsyncParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
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

// toGeneratedRunRequest converts a RunRequest to the generated model for API calls.
// It maps all Claude and Podman options to their corresponding generated types.
func toGeneratedRunRequest(req *RunRequest) *models.RunRequest {
	prompt := req.Prompt
	genReq := &models.RunRequest{
		Prompt:     &prompt,
		Workdir:    req.Workdir,
		WebhookURL: req.WebhookURL,
	}

	// Convert Claude options - only populate if user provided them
	if req.Claude != nil {
		genReq.Claude = models.StromboliInternalTypesClaudeOptions{
			Model:                           string(req.Claude.Model),
			SessionID:                       req.Claude.SessionID,
			Resume:                          req.Claude.Resume,
			MaxBudgetUsd:                    req.Claude.MaxBudgetUSD,
			SystemPrompt:                    req.Claude.SystemPrompt,
			AppendSystemPrompt:              req.Claude.AppendSystemPrompt,
			AllowedTools:                    req.Claude.AllowedTools,
			DisallowedTools:                 req.Claude.DisallowedTools,
			DangerouslySkipPermissions:      req.Claude.DangerouslySkipPermissions,
			PermissionMode:                  req.Claude.PermissionMode,
			OutputFormat:                    req.Claude.OutputFormat,
			JSONSchema:                      req.Claude.JSONSchema,
			Verbose:                         req.Claude.Verbose,
			Debug:                           req.Claude.Debug,
			Continue:                        req.Claude.Continue,
			Agent:                           req.Claude.Agent,
			FallbackModel:                   req.Claude.FallbackModel,
			AddDirs:                         req.Claude.AddDirs,
			Agents:                          req.Claude.Agents,
			AllowDangerouslySkipPermissions: req.Claude.AllowDangerouslySkipPermissions,
			Betas:                           req.Claude.Betas,
			DisableSlashCommands:            req.Claude.DisableSlashCommands,
			Files:                           req.Claude.Files,
			ForkSession:                     req.Claude.ForkSession,
			IncludePartialMessages:          req.Claude.IncludePartialMessages,
			InputFormat:                     req.Claude.InputFormat,
			McpConfigs:                      req.Claude.McpConfigs,
			NoPersistence:                   req.Claude.NoPersistence,
			PluginDirs:                      req.Claude.PluginDirs,
			ReplayUserMessages:              req.Claude.ReplayUserMessages,
			SettingSources:                  req.Claude.SettingSources,
			Settings:                        req.Claude.Settings,
			StrictMcpConfig:                 req.Claude.StrictMcpConfig,
			Tools:                           req.Claude.Tools,
		}
	}

	// Convert Podman options - only populate if user provided them
	if req.Podman != nil {
		genReq.Podman = models.StromboliInternalTypesPodmanOptions{
			Memory:     req.Podman.Memory,
			Timeout:    req.Podman.Timeout,
			Cpus:       req.Podman.Cpus,
			CPUShares:  req.Podman.CPUShares,
			Volumes:    req.Podman.Volumes,
			Image:      req.Podman.Image,
			SecretsEnv: req.Podman.SecretsEnv,
		}

		// Only set nested structs if user provided them
		if req.Podman.Lifecycle != nil {
			genReq.Podman.Lifecycle = models.StromboliInternalTypesLifecycleHooks{
				OnCreateCommand: req.Podman.Lifecycle.OnCreateCommand,
				PostCreate:      req.Podman.Lifecycle.PostCreate,
				PostStart:       req.Podman.Lifecycle.PostStart,
				HooksTimeout:    req.Podman.Lifecycle.HooksTimeout,
			}
		}

		// Only set environment config if user provided it
		if req.Podman.Environment != nil {
			genReq.Podman.Environment = models.StromboliInternalTypesEnvironmentConfig{
				Type:         req.Podman.Environment.Type,
				Path:         req.Podman.Environment.Path,
				Service:      req.Podman.Environment.Service,
				BuildTimeout: req.Podman.Environment.BuildTimeout,
			}
		}
	}

	return genReq
}

// ----------------------------------------------------------------------------
// Job Methods
// ----------------------------------------------------------------------------

// ListJobs returns all async jobs.
//
// Use this method to get an overview of all jobs, their status, and
// when they were created. The list includes pending, running, completed,
// failed, and cancelled jobs.
//
// Example:
//
//	jobs, err := client.ListJobs(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, job := range jobs {
//	    fmt.Printf("%s: %s (created: %s)\n", job.ID, job.Status, job.CreatedAt)
//	}
//
// Filter by status:
//
//	jobs, _ := client.ListJobs(ctx)
//	for _, job := range jobs {
//	    if job.IsRunning() {
//	        fmt.Printf("Job %s is still running\n", job.ID)
//	    }
//	}
func (c *Client) ListJobs(ctx context.Context) ([]*Job, error) {
	// Create request parameters with context
	params := jobs.NewGetJobsParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))

	// Execute request
	resp, err := c.api.Jobs.GetJobs(params)
	if err != nil {
		return nil, c.handleError(err, "failed to list jobs")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty jobs list response", 0, nil)
	}

	// Map jobs
	result := make([]*Job, 0, len(payload.Jobs))
	for _, j := range payload.Jobs {
		if j != nil {
			result = append(result, fromGeneratedJobResponse(j))
		}
	}

	return result, nil
}

// GetJob returns the status and result of an async job.
//
// Use this method to poll for job completion or check the status of
// a previously started async execution.
//
// Basic polling example:
//
//	job, _ := client.RunAsync(ctx, req)
//
//	for {
//	    status, err := client.GetJob(ctx, job.JobID)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    switch {
//	    case status.IsCompleted():
//	        fmt.Println(status.Output)
//	        return
//	    case status.IsFailed():
//	        log.Fatalf("Job failed: %s", status.Error)
//	    case status.IsRunning():
//	        fmt.Println("Still running...")
//	        time.Sleep(2 * time.Second)
//	    }
//	}
//
// Returns [ErrNotFound] if the job doesn't exist:
//
//	status, err := client.GetJob(ctx, "invalid-id")
//	if errors.Is(err, stromboli.ErrNotFound) {
//	    fmt.Println("Job not found")
//	}
func (c *Client) GetJob(ctx context.Context, jobID string) (*Job, error) {
	if jobID == "" {
		return nil, newError("BAD_REQUEST", "job ID is required", 400, nil)
	}

	// Create request parameters with context
	params := jobs.NewGetJobsIDParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetID(jobID)

	// Execute request
	resp, err := c.api.Jobs.GetJobsID(params)
	if err != nil {
		return nil, c.handleError(err, "failed to get job")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty job response", 0, nil)
	}

	return fromGeneratedJobResponse(payload), nil
}

// CancelJob cancels a pending or running job.
//
// Use this method to stop a job that is no longer needed. Only pending
// and running jobs can be cancelled. Completed, failed, or already
// cancelled jobs cannot be cancelled (returns 409 Conflict error).
//
// Example:
//
//	err := client.CancelJob(ctx, "job-abc123")
//	if err != nil {
//	    if errors.Is(err, stromboli.ErrNotFound) {
//	        fmt.Println("Job not found")
//	    } else {
//	        log.Fatal(err)
//	    }
//	}
//	fmt.Println("Job cancelled successfully")
//
// Cancel a job immediately after starting:
//
//	job, _ := client.RunAsync(ctx, req)
//
//	// Changed our mind, cancel it
//	err := client.CancelJob(ctx, job.JobID)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) CancelJob(ctx context.Context, jobID string) error {
	if jobID == "" {
		return newError("BAD_REQUEST", "job ID is required", 400, nil)
	}

	// Create request parameters with context
	params := jobs.NewDeleteJobsIDParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetID(jobID)

	// Execute request
	_, err := c.api.Jobs.DeleteJobsID(params)
	if err != nil {
		return c.handleError(err, "failed to cancel job")
	}

	return nil
}

// fromGeneratedJobResponse converts a generated JobResponse model to the SDK Job type.
// It handles the mapping of all fields including optional crash info.
func fromGeneratedJobResponse(j *models.JobResponse) *Job {
	job := &Job{
		ID:        j.ID,
		Status:    string(j.Status),
		Output:    j.Output,
		Error:     j.Error,
		SessionID: j.SessionID,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}

	// Convert crash info if present
	if j.CrashInfo != nil {
		job.CrashInfo = &CrashInfo{
			Reason:        j.CrashInfo.Reason,
			ExitCode:      j.CrashInfo.ExitCode,
			PartialOutput: j.CrashInfo.PartialOutput,
			Signal:        j.CrashInfo.Signal,
			TaskCompleted: j.CrashInfo.TaskCompleted,
		}
	}

	return job
}

// ----------------------------------------------------------------------------
// Session Methods
// ----------------------------------------------------------------------------

// ListSessions returns all existing session IDs.
//
// Sessions are created automatically when running Claude with a new
// conversation. Use this method to list all available sessions for
// resumption or cleanup.
//
// Example:
//
//	sessionIDs, err := client.ListSessions(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, id := range sessionIDs {
//	    fmt.Printf("Session: %s\n", id)
//	}
//
// To continue a specific session, use the session ID with [ClaudeOptions.SessionID]:
//
//	result, _ := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt: "What did we discuss earlier?",
//	    Claude: &stromboli.ClaudeOptions{
//	        SessionID: sessionIDs[0],
//	        Resume:    true,
//	    },
//	})
func (c *Client) ListSessions(ctx context.Context) ([]string, error) {
	// Create request parameters with context
	params := sessions.NewGetSessionsParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))

	// Execute request
	resp, err := c.api.Sessions.GetSessions(params)
	if err != nil {
		return nil, c.handleError(err, "failed to list sessions")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty sessions list response", 0, nil)
	}

	return payload.Sessions, nil
}

// DestroySession removes a session and all its stored data.
//
// Use this method to clean up old sessions that are no longer needed.
// This operation is permanent and cannot be undone.
//
// Example:
//
//	err := client.DestroySession(ctx, "sess-abc123")
//	if err != nil {
//	    if errors.Is(err, stromboli.ErrNotFound) {
//	        fmt.Println("Session not found")
//	    } else {
//	        log.Fatal(err)
//	    }
//	}
//	fmt.Println("Session destroyed")
//
// Bulk cleanup:
//
//	sessions, _ := client.ListSessions(ctx)
//	for _, id := range sessions {
//	    if err := client.DestroySession(ctx, id); err != nil {
//	        log.Printf("Failed to destroy %s: %v\n", id, err)
//	    }
//	}
func (c *Client) DestroySession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return newError("BAD_REQUEST", "session ID is required", 400, nil)
	}

	// Create request parameters with context
	params := sessions.NewDeleteSessionsIDParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetID(sessionID)

	// Execute request
	_, err := c.api.Sessions.DeleteSessionsID(params)
	if err != nil {
		return c.handleError(err, "failed to destroy session")
	}

	return nil
}

// GetMessages returns paginated conversation history for a session.
//
// Use this method to retrieve past messages from a session, including
// user prompts, assistant responses, tool calls, and results.
//
// Basic usage:
//
//	messages, err := client.GetMessages(ctx, "sess-abc123", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, msg := range messages.Messages {
//	    fmt.Printf("[%s] %s\n", msg.Type, msg.UUID)
//	}
//
// With pagination:
//
//	messages, _ := client.GetMessages(ctx, "sess-abc123", &stromboli.GetMessagesOptions{
//	    Limit:  50,
//	    Offset: 100,
//	})
//
//	if messages.HasMore {
//	    // Fetch next page
//	    nextPage, _ := client.GetMessages(ctx, "sess-abc123", &stromboli.GetMessagesOptions{
//	        Limit:  50,
//	        Offset: messages.Offset + messages.Limit,
//	    })
//	}
func (c *Client) GetMessages(ctx context.Context, sessionID string, opts *GetMessagesOptions) (*MessagesResponse, error) {
	if sessionID == "" {
		return nil, newError("BAD_REQUEST", "session ID is required", 400, nil)
	}

	// Create request parameters with context
	params := sessions.NewGetSessionsIDMessagesParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetID(sessionID)

	// Apply options if provided
	if opts != nil {
		// Validate negative values - catch client-side for better error messages
		if opts.Limit < 0 {
			return nil, newError("BAD_REQUEST", "limit cannot be negative", 400, nil)
		}
		if opts.Offset < 0 {
			return nil, newError("BAD_REQUEST", "offset cannot be negative", 400, nil)
		}
		if opts.Limit > 0 {
			params.SetLimit(&opts.Limit)
		}
		// Note: Offset == 0 is valid (start from beginning), so > 0 check is correct
		if opts.Offset > 0 {
			params.SetOffset(&opts.Offset)
		}
	}

	// Execute request
	resp, err := c.api.Sessions.GetSessionsIDMessages(params)
	if err != nil {
		return nil, c.handleError(err, "failed to get messages")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty messages response", 0, nil)
	}

	// Map messages
	messages := make([]*Message, 0, len(payload.Messages))
	for _, m := range payload.Messages {
		if m != nil {
			messages = append(messages, fromGeneratedMessage(m))
		}
	}

	return &MessagesResponse{
		Messages: messages,
		Total:    payload.Total,
		Limit:    payload.Limit,
		Offset:   payload.Offset,
		HasMore:  payload.HasMore,
	}, nil
}

// GetMessage returns a specific message from session history by UUID.
//
// Use this method to retrieve full details about a specific message,
// including its content, tool calls, and results.
//
// Example:
//
//	msg, err := client.GetMessage(ctx, "sess-abc123", "msg-uuid-456")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Role: %s\n", msg.Type)
//	fmt.Printf("Content: %v\n", msg.Content)
func (c *Client) GetMessage(ctx context.Context, sessionID, messageID string) (*Message, error) {
	if sessionID == "" {
		return nil, newError("BAD_REQUEST", "session ID is required", 400, nil)
	}
	if messageID == "" {
		return nil, newError("BAD_REQUEST", "message ID is required", 400, nil)
	}

	// Create request parameters with context
	params := sessions.NewGetSessionsIDMessagesMessageIDParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetID(sessionID)
	params.SetMessageID(messageID)

	// Execute request
	resp, err := c.api.Sessions.GetSessionsIDMessagesMessageID(params)
	if err != nil {
		return nil, c.handleError(err, "failed to get message")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil || payload.Message == nil {
		return nil, newError("INVALID_RESPONSE", "empty message response", 0, nil)
	}

	return fromGeneratedMessage(payload.Message), nil
}

// fromGeneratedMessage converts a generated message model to the SDK Message type.
// Note: Content and ToolResult are exposed as interface{} for flexibility.
func fromGeneratedMessage(m *models.StromboliInternalHistoryMessage) *Message {
	return &Message{
		UUID:           m.UUID,
		Type:           string(m.Type),
		ParentUUID:     m.ParentUUID,
		SessionID:      m.SessionID,
		Cwd:            m.Cwd,
		GitBranch:      m.GitBranch,
		PermissionMode: m.PermissionMode,
		Timestamp:      m.Timestamp,
		Version:        m.Version,
		// Note: Content and ToolResult are complex nested structures.
		// We expose them as interface{} for flexibility.
		Content:    m.Content,
		ToolResult: m.ToolResult,
	}
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
	var apiErr *runtime.APIError
	if errors.As(err, &apiErr) {
		return c.handleAPIError(apiErr, message)
	}

	// Check for context cancellation
	if errors.Is(err, context.Canceled) {
		return wrapError(err, "CANCELLED", "request was cancelled", 0)
	}

	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return wrapError(err, "TIMEOUT", "request timed out", http.StatusRequestTimeout)
	}

	// Generic error
	return wrapError(err, "REQUEST_FAILED", message, 0)
}

// httpStatusToErrorCode maps HTTP status codes to error codes for table-driven error handling.
var httpStatusToErrorCode = map[int]string{
	http.StatusBadRequest:          ErrBadRequest.Code,
	http.StatusUnauthorized:        ErrUnauthorized.Code,
	http.StatusForbidden:           "FORBIDDEN",
	http.StatusNotFound:            ErrNotFound.Code,
	http.StatusConflict:            "CONFLICT",
	http.StatusRequestTimeout:      ErrTimeout.Code,
	http.StatusTooManyRequests:     ErrRateLimited.Code,
	http.StatusServiceUnavailable:  ErrUnavailable.Code,
	http.StatusInternalServerError: ErrInternal.Code,
}

// handleAPIError converts go-swagger API errors into SDK errors.
// It wraps sentinel errors so that errors.Is works consistently.
// The original server error message is preserved in the Cause chain.
func (c *Client) handleAPIError(apiErr *runtime.APIError, fallbackMsg string) error {
	status := apiErr.Code

	// Extract the most useful error message:
	// 1. Try the API error's message
	// 2. Fall back to the provided fallback message
	serverMsg := fallbackMsg
	if msg := apiErr.Error(); msg != "" && msg != fmt.Sprintf("[%d] ", status) {
		serverMsg = msg
	}

	// Look up error code in table
	if code, ok := httpStatusToErrorCode[status]; ok {
		return wrapError(apiErr, code, serverMsg, status)
	}

	// Handle other 5xx errors
	if status >= http.StatusInternalServerError {
		return wrapError(apiErr, ErrInternal.Code, serverMsg, status)
	}

	return newError("REQUEST_FAILED", serverMsg, status, apiErr)
}

// ----------------------------------------------------------------------------
// Auth Methods
// ----------------------------------------------------------------------------

// bearerAuth returns a runtime.ClientAuthInfoWriter for Bearer token auth.
//
// The token is read at the time the request is authenticated, not when
// this method is called. This ensures the most current token is used,
// which is important if SetToken is called between method calls.
func (c *Client) bearerAuth() runtime.ClientAuthInfoWriter {
	return runtime.ClientAuthInfoWriterFunc(func(r runtime.ClientRequest, _ strfmt.Registry) error {
		token := c.getToken() // Read at write time
		if token != "" {
			return r.SetHeaderParam("Authorization", "Bearer "+token)
		}
		return nil
	})
}

// getToken returns the current token (thread-safe).
func (c *Client) getToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// SetToken sets the Bearer token for authenticated requests.
//
// This token is used for endpoints that require authentication,
// such as [Client.ValidateToken] and [Client.Logout].
// SetToken is safe for concurrent use.
//
// Example:
//
//	tokens, _ := client.GetToken(ctx, "my-client-id")
//	client.SetToken(tokens.AccessToken)
//
//	// Now authenticated endpoints will work
//	validation, _ := client.ValidateToken(ctx)
func (c *Client) SetToken(token string) {
	// Validate token to prevent HTTP header injection via CR/LF characters.
	// Empty string is valid (clears token), but non-empty tokens must be safe.
	if token != "" && !isValidToken(token) {
		getLogger().Printf("stromboli: WARNING: SetToken called with invalid token (contains control characters), ignoring")
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

// ClearToken removes the Bearer token from the client.
//
// This is equivalent to calling SetToken("") but more explicit.
// Use this after [Client.Logout] to clear local state.
//
// Example:
//
//	client.Logout(ctx)
//	client.ClearToken()
func (c *Client) ClearToken() {
	c.SetToken("")
}

// GetToken obtains JWT tokens using a client ID.
//
// Use this method to authenticate with the Stromboli API. The returned
// tokens can be used for subsequent authenticated requests.
//
// Example:
//
//	tokens, err := client.GetToken(ctx, "my-client-id")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Set the token for future requests
//	client.SetToken(tokens.AccessToken)
//
//	// Token expires in tokens.ExpiresIn seconds
//	fmt.Printf("Token expires in %d seconds\n", tokens.ExpiresIn)
func (c *Client) GetToken(ctx context.Context, clientID string) (*TokenResponse, error) {
	if clientID == "" {
		return nil, newError("BAD_REQUEST", "client ID is required", 400, nil)
	}

	// Create request parameters
	params := auth.NewPostAuthTokenParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetRequest(&models.TokenRequest{
		ClientID: &clientID,
	})

	// Execute request - GetToken doesn't require authentication
	// Pass nil auth writer to avoid sending empty Bearer header
	resp, err := c.api.Auth.PostAuthToken(params, nil)
	if err != nil {
		return nil, c.handleError(err, "failed to get token")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty token response", 0, nil)
	}

	return &TokenResponse{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresIn:    payload.ExpiresIn,
		TokenType:    payload.TokenType,
	}, nil
}

// RefreshToken obtains a new access token using a refresh token.
//
// Use this method when your access token has expired. The refresh
// token has a longer lifetime and can be used to obtain new access tokens.
//
// Example:
//
//	// When access token expires, use refresh token
//	newTokens, err := client.RefreshToken(ctx, tokens.RefreshToken)
//	if err != nil {
//	    // Refresh token may also be expired, need to re-authenticate
//	    log.Fatal(err)
//	}
//
//	client.SetToken(newTokens.AccessToken)
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	if refreshToken == "" {
		return nil, newError("BAD_REQUEST", "refresh token is required", 400, nil)
	}

	// Create request parameters
	params := auth.NewPostAuthRefreshParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetRequest(&models.RefreshRequest{
		RefreshToken: &refreshToken,
	})

	// Execute request (no auth required for refresh)
	resp, err := c.api.Auth.PostAuthRefresh(params)
	if err != nil {
		return nil, c.handleError(err, "failed to refresh token")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty token response", 0, nil)
	}

	return &TokenResponse{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresIn:    payload.ExpiresIn,
		TokenType:    payload.TokenType,
	}, nil
}

// ValidateToken validates the current access token and returns its claims.
//
// This method requires a valid token to be set using [Client.SetToken].
//
// Example:
//
//	client.SetToken(accessToken)
//
//	validation, err := client.ValidateToken(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if validation.Valid {
//	    fmt.Printf("Token valid for subject: %s\n", validation.Subject)
//	    fmt.Printf("Expires at: %d\n", validation.ExpiresAt)
//	}
func (c *Client) ValidateToken(ctx context.Context) (*TokenValidation, error) {
	if c.getToken() == "" {
		return nil, newError("UNAUTHORIZED", "no token set, use SetToken() first", 401, nil)
	}

	// Create request parameters
	params := auth.NewGetAuthValidateParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))

	// Execute request with bearer auth
	resp, err := c.api.Auth.GetAuthValidate(params, c.bearerAuth())
	if err != nil {
		return nil, c.handleError(err, "failed to validate token")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty validation response", 0, nil)
	}

	return &TokenValidation{
		Valid:     payload.Valid,
		Subject:   payload.Subject,
		ExpiresAt: payload.ExpiresAt,
	}, nil
}

// Logout invalidates the current access token.
//
// After calling this method, the token will no longer be accepted by the API.
// This method requires a valid token to be set using [Client.SetToken].
//
// Example:
//
//	client.SetToken(accessToken)
//
//	result, err := client.Logout(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if result.Success {
//	    fmt.Println("Successfully logged out")
//	    client.SetToken("") // Clear the token
//	}
func (c *Client) Logout(ctx context.Context) (*LogoutResponse, error) {
	if c.getToken() == "" {
		return nil, newError("UNAUTHORIZED", "no token set, use SetToken() first", 401, nil)
	}

	// Create request parameters
	params := auth.NewPostAuthLogoutParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))

	// Execute request with bearer auth
	resp, err := c.api.Auth.PostAuthLogout(params, c.bearerAuth())
	if err != nil {
		return nil, c.handleError(err, "failed to logout")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty logout response", 0, nil)
	}

	return &LogoutResponse{
		Success: payload.Success,
		Message: payload.Message,
	}, nil
}

// ----------------------------------------------------------------------------
// Secrets Methods
// ----------------------------------------------------------------------------

// ListSecrets returns all available Podman secrets.
//
// These secrets can be injected into container execution environments
// using [PodmanOptions.SecretsEnv].
//
// Example:
//
//	secrets, err := client.ListSecrets(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, s := range secrets {
//	    fmt.Printf("Secret: %s (created: %s)\n", s.Name, s.CreatedAt)
//	}
//
// Using secrets in execution:
//
//	secrets, _ := client.ListSecrets(ctx)
//
//	result, _ := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt: "Use my GitHub token to list repos",
//	    Podman: &stromboli.PodmanOptions{
//	        SecretsEnv: map[string]string{
//	            "GITHUB_TOKEN": secrets[0].Name, // Use first available secret
//	        },
//	    },
//	})
func (c *Client) ListSecrets(ctx context.Context) ([]*Secret, error) {
	// Create request parameters
	params := secrets.NewGetSecretsParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))

	// Execute request
	resp, err := c.api.Secrets.GetSecrets(params)
	if err != nil {
		return nil, c.handleError(err, "failed to list secrets")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty secrets response", 0, nil)
	}

	// Check for error in response
	if payload.Error != "" {
		return nil, newError("SECRETS_ERROR", payload.Error, 500, nil)
	}

	// Map secrets
	result := make([]*Secret, 0, len(payload.Secrets))
	for _, s := range payload.Secrets {
		if s != nil {
			result = append(result, &Secret{
				ID:        s.ID,
				Name:      s.Name,
				CreatedAt: s.CreatedAt,
			})
		}
	}

	return result, nil
}

// CreateSecret creates a new Podman secret.
//
// Secrets can be used to securely pass sensitive data (API keys, tokens, etc.)
// to containers without exposing them in environment variables or command lines.
//
// Example:
//
//	err := client.CreateSecret(ctx, &stromboli.CreateSecretRequest{
//	    Name:  "github-token",
//	    Value: "ghp_xxxx...",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Returns [ErrSecretExists] if a secret with this name already exists:
//
//	err := client.CreateSecret(ctx, req)
//	if errors.Is(err, stromboli.ErrSecretExists) {
//	    fmt.Println("Secret already exists")
//	}
func (c *Client) CreateSecret(ctx context.Context, req *CreateSecretRequest) error {
	if req == nil {
		return newError("BAD_REQUEST", "request is required", 400, nil)
	}
	if req.Name == "" {
		return newError("BAD_REQUEST", "secret name is required", 400, nil)
	}
	if req.Value == "" {
		return newError("BAD_REQUEST", "secret value is required", 400, nil)
	}

	// Create request parameters
	params := secrets.NewPostSecretsParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetRequest(&models.CreateSecretRequest{
		Name:  &req.Name,
		Value: &req.Value,
	})

	// Execute request
	resp, err := c.api.Secrets.PostSecrets(params)
	if err != nil {
		// Check for conflict (secret already exists)
		var apiErr *runtime.APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusConflict {
			return ErrSecretExists
		}
		return c.handleError(err, "failed to create secret")
	}

	// Check response
	payload := resp.GetPayload()
	if payload == nil {
		return newError("INVALID_RESPONSE", "empty secret response", 0, nil)
	}

	// Check for error in response
	if payload.Error != "" {
		return newError("SECRET_CREATE_ERROR", payload.Error, 500, nil)
	}

	if !payload.Success {
		return newError("SECRET_CREATE_FAILED", "failed to create secret", 500, nil)
	}

	return nil
}

// GetSecret retrieves metadata for a specific secret.
//
// For security, the actual secret value is never returned - only the ID,
// name, and creation time.
//
// Example:
//
//	secret, err := client.GetSecret(ctx, "github-token")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Secret %s created at %s\n", secret.Name, secret.CreatedAt)
//
// Returns [ErrNotFound] if the secret doesn't exist:
//
//	secret, err := client.GetSecret(ctx, "unknown-secret")
//	if errors.Is(err, stromboli.ErrNotFound) {
//	    fmt.Println("Secret not found")
//	}
func (c *Client) GetSecret(ctx context.Context, name string) (*Secret, error) {
	if name == "" {
		return nil, newError("BAD_REQUEST", "secret name is required", 400, nil)
	}

	// Create request parameters
	params := secrets.NewGetSecretsNameParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetName(name)

	// Execute request
	resp, err := c.api.Secrets.GetSecretsName(params)
	if err != nil {
		// Check for not found
		var apiErr *runtime.APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, c.handleError(err, "failed to get secret")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty secret response", 0, nil)
	}

	return &Secret{
		ID:        payload.ID,
		Name:      payload.Name,
		CreatedAt: payload.CreatedAt,
	}, nil
}

// DeleteSecret permanently deletes a Podman secret.
//
// WARNING: This action cannot be undone. Secrets currently in use by
// running containers may cause those containers to fail.
//
// Example:
//
//	err := client.DeleteSecret(ctx, "github-token")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Secret deleted")
//
// Returns [ErrNotFound] if the secret doesn't exist:
//
//	err := client.DeleteSecret(ctx, "unknown-secret")
//	if errors.Is(err, stromboli.ErrNotFound) {
//	    fmt.Println("Secret not found")
//	}
func (c *Client) DeleteSecret(ctx context.Context, name string) error {
	if name == "" {
		return newError("BAD_REQUEST", "secret name is required", 400, nil)
	}

	// Create request parameters
	params := secrets.NewDeleteSecretsNameParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetName(name)

	// Execute request
	_, err := c.api.Secrets.DeleteSecretsName(params)
	if err != nil {
		// Check for not found
		var apiErr *runtime.APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
			return ErrNotFound
		}
		return c.handleError(err, "failed to delete secret")
	}

	return nil
}

// ----------------------------------------------------------------------------
// Images Methods
// ----------------------------------------------------------------------------

// ListImages returns all local container images sorted by compatibility rank.
//
// Images are ranked by their compatibility with Stromboli:
//   - Rank 1-2: Verified compatible (have required tools)
//   - Rank 3: Standard glibc (compatible)
//   - Rank 4: Incompatible (Alpine/musl)
//
// Example:
//
//	images, err := client.ListImages(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, img := range images {
//	    fmt.Printf("%s:%s (rank %d, compatible: %v)\n",
//	        img.Repository, img.Tag, img.CompatibilityRank, img.Compatible)
//	}
func (c *Client) ListImages(ctx context.Context) ([]*Image, error) {
	// Create request parameters
	params := images.NewGetImagesParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))

	// Execute request
	resp, err := c.api.Images.GetImages(params)
	if err != nil {
		return nil, c.handleError(err, "failed to list images")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty images response", 0, nil)
	}

	// Map images
	result := make([]*Image, 0, len(payload.Images))
	for _, img := range payload.Images {
		if img != nil {
			result = append(result, fromGeneratedImage(img))
		}
	}

	return result, nil
}

// GetImage returns detailed information about a specific container image.
//
// This includes all labels, compatibility information, and available tools.
//
// Example:
//
//	image, err := client.GetImage(ctx, "python:3.12-slim")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Image: %s\n", image.ID)
//	fmt.Printf("Compatible: %v\n", image.Compatible)
//	fmt.Printf("Tools: %v\n", image.Tools)
//
// Returns [ErrImageNotFound] if the image doesn't exist locally:
//
//	image, err := client.GetImage(ctx, "nonexistent:latest")
//	if errors.Is(err, stromboli.ErrImageNotFound) {
//	    fmt.Println("Image not found")
//	}
func (c *Client) GetImage(ctx context.Context, name string) (*Image, error) {
	if name == "" {
		return nil, newError("BAD_REQUEST", "image name is required", 400, nil)
	}

	// Create request parameters
	params := images.NewGetImagesNameParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetName(name)

	// Execute request
	resp, err := c.api.Images.GetImagesName(params)
	if err != nil {
		// Check for not found
		var apiErr *runtime.APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
			return nil, ErrImageNotFound
		}
		return nil, c.handleError(err, "failed to get image")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty image response", 0, nil)
	}

	return fromGeneratedImageDetail(payload), nil
}

// SearchImages searches container registries for images matching the query.
//
// Returns results from Docker Hub and other configured registries.
//
// Example:
//
//	results, err := client.SearchImages(ctx, &stromboli.SearchImagesOptions{
//	    Query: "python",
//	    Limit: 10,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, r := range results {
//	    fmt.Printf("%s: %s (stars: %d, official: %v)\n",
//	        r.Name, r.Description, r.Stars, r.Official)
//	}
func (c *Client) SearchImages(ctx context.Context, opts *SearchImagesOptions) ([]*ImageSearchResult, error) {
	if opts == nil || opts.Query == "" {
		return nil, newError("BAD_REQUEST", "search query is required", 400, nil)
	}

	// Create request parameters
	params := images.NewGetImagesSearchParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))
	params.SetQ(opts.Query)

	if opts.Limit > 0 {
		params.SetLimit(&opts.Limit)
	}
	if opts.NoTrunc {
		params.SetNoTrunc(&opts.NoTrunc)
	}

	// Execute request
	resp, err := c.api.Images.GetImagesSearch(params)
	if err != nil {
		return nil, c.handleError(err, "failed to search images")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty search response", 0, nil)
	}

	// Map results
	results := make([]*ImageSearchResult, 0, len(payload.Results))
	for _, r := range payload.Results {
		if r != nil {
			results = append(results, &ImageSearchResult{
				Name:        r.Name,
				Description: r.Description,
				Stars:       r.Stars,
				Official:    r.Official,
				Automated:   r.Automated,
				Index:       r.Index,
			})
		}
	}

	return results, nil
}

// PullImage pulls a container image from a registry.
//
// This operation may take some time for large images.
//
// Example:
//
//	result, err := client.PullImage(ctx, &stromboli.PullImageRequest{
//	    Image:    "python:3.12-slim",
//	    Platform: "linux/amd64",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if result.Success {
//	    fmt.Printf("Pulled image %s (ID: %s)\n", result.Image, result.ImageID)
//	}
func (c *Client) PullImage(ctx context.Context, req *PullImageRequest) (*PullImageResponse, error) {
	if req == nil {
		return nil, newError("BAD_REQUEST", "request is required", 400, nil)
	}
	if req.Image == "" {
		return nil, newError("BAD_REQUEST", "image name is required", 400, nil)
	}

	// Create request parameters
	params := images.NewPostImagesPullParams()
	params.SetContext(ctx)
	params.SetTimeout(c.effectiveTimeout(ctx))

	image := req.Image
	params.SetRequest(&models.ImagePullRequest{
		Image:    &image,
		Platform: req.Platform,
		Quiet:    req.Quiet,
	})

	// Execute request
	resp, err := c.api.Images.PostImagesPull(params)
	if err != nil {
		return nil, c.handleError(err, "failed to pull image")
	}

	// Convert response
	payload := resp.GetPayload()
	if payload == nil {
		return nil, newError("INVALID_RESPONSE", "empty pull response", 0, nil)
	}

	return &PullImageResponse{
		Success: payload.Success,
		Image:   payload.Image,
		ImageID: payload.ImageID,
	}, nil
}

// fromGeneratedImage converts a generated ImageInfoResponse to our Image type.
func fromGeneratedImage(img *models.ImageInfoResponse) *Image {
	return &Image{
		ID:                img.ID,
		Repository:        img.Repository,
		Tag:               img.Tag,
		Size:              img.Size,
		Created:           img.Created,
		Description:       img.Description,
		Compatible:        img.Compatible,
		CompatibilityRank: img.CompatibilityRank,
		HasClaudeCLI:      img.HasClaudeCli,
		Tools:             img.Tools,
	}
}

// fromGeneratedImageDetail converts a generated ImageDetailResponse to our Image type.
func fromGeneratedImageDetail(img *models.ImageDetailResponse) *Image {
	return &Image{
		ID:                img.ID,
		Repository:        img.Repository,
		Tag:               img.Tag,
		Size:              img.Size,
		Created:           img.Created,
		Description:       img.Description,
		Compatible:        img.Compatible,
		CompatibilityRank: img.CompatibilityRank,
		HasClaudeCLI:      img.HasClaudeCli,
		Tools:             img.Tools,
	}
}

// validateRequestSize checks that request fields don't exceed size limits.
// This prevents memory exhaustion from excessively large requests.
func validateRequestSize(req *RunRequest) error {
	if len(req.Prompt) > maxPromptSize {
		return newError("BAD_REQUEST",
			fmt.Sprintf("prompt exceeds maximum size of %d bytes (got %d)", maxPromptSize, len(req.Prompt)),
			400, nil)
	}
	if req.Claude != nil {
		if len(req.Claude.SystemPrompt) > maxSystemPromptSize {
			return newError("BAD_REQUEST",
				fmt.Sprintf("system prompt exceeds maximum size of %d bytes (got %d)", maxSystemPromptSize, len(req.Claude.SystemPrompt)),
				400, nil)
		}
		if len(req.Claude.JSONSchema) > maxJSONSchemaSize {
			return newError("BAD_REQUEST",
				fmt.Sprintf("JSON schema exceeds maximum size of %d bytes (got %d)", maxJSONSchemaSize, len(req.Claude.JSONSchema)),
				400, nil)
		}
	}
	return nil
}

// validateJSONSchema performs MINIMAL validation of a JSON schema string.
//
// WARNING: This does NOT validate JSON Schema compliance. It only checks:
//   - The string is valid JSON
//   - At least one recognized schema keyword exists
//
// Invalid schemas WILL pass this check and fail server-side.
// For production use, pre-validate schemas with a JSON Schema library such as:
//   - github.com/santhosh-tekuri/jsonschema
//   - github.com/xeipuuv/gojsonschema
func validateJSONSchema(schema string) error {
	// Parse the schema (single parse instead of json.Valid + Unmarshal)
	var s map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &s); err != nil {
		return fmt.Errorf("not valid JSON: %w", err)
	}

	// Check for at least one valid JSON Schema keyword.
	// This list covers the most common structural keywords from JSON Schema
	// draft-07 and later. It's intentionally broad to avoid rejecting
	// valid schemas while still catching obvious non-schemas like {"foo": 1}.
	validKeywords := []string{
		// Type keywords
		"type", "$ref", "oneOf", "anyOf", "allOf", "enum", "const",
		// Object keywords
		"properties", "required", "additionalProperties", "patternProperties",
		// Array keywords
		"items", "additionalItems", "contains",
		// Schema composition
		"definitions", "$defs", "not", "if", "then", "else",
		// Validation keywords
		"minimum", "maximum", "minLength", "maxLength", "pattern",
		"minItems", "maxItems", "uniqueItems",
		"minProperties", "maxProperties",
	}
	for _, keyword := range validKeywords {
		if _, ok := s[keyword]; ok {
			return nil
		}
	}

	return fmt.Errorf("schema must contain at least one JSON Schema keyword (type, properties, items, etc.)")
}

// isValidTokenChar returns true if the token contains only valid HTTP header characters.
// This prevents HTTP header injection attacks via malicious tokens containing CR/LF.
func isValidToken(token string) bool {
	for _, c := range token {
		// Reject CR, LF, and other control characters that could enable header injection
		if c == '\r' || c == '\n' || c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}
