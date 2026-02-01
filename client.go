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
	"github.com/tomblancdev/stromboli-go/generated/client/auth"
	"github.com/tomblancdev/stromboli-go/generated/client/execution"
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

	// maxRetries is the maximum number of retry attempts.
	maxRetries int

	// userAgent is the User-Agent header value.
	userAgent string

	// token is the Bearer token for authenticated requests.
	token string

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
	params.SetTimeout(c.timeout)

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
			result = append(result, c.fromGeneratedJobResponse(j))
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
	params.SetTimeout(c.timeout)
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

	return c.fromGeneratedJobResponse(payload), nil
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
	params.SetTimeout(c.timeout)
	params.SetID(jobID)

	// Execute request
	_, err := c.api.Jobs.DeleteJobsID(params)
	if err != nil {
		return c.handleError(err, "failed to cancel job")
	}

	return nil
}

// fromGeneratedJobResponse converts a generated JobResponse to our Job type.
func (c *Client) fromGeneratedJobResponse(j *models.JobResponse) *Job {
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
	params.SetTimeout(c.timeout)

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
	params.SetTimeout(c.timeout)
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
//	    fmt.Printf("[%s] %s\n", msg.Role, msg.UUID)
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
	params.SetTimeout(c.timeout)
	params.SetID(sessionID)

	// Apply options if provided
	if opts != nil {
		if opts.Limit > 0 {
			params.SetLimit(&opts.Limit)
		}
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
			messages = append(messages, c.fromGeneratedMessage(m))
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
//	fmt.Printf("Role: %s\n", msg.Role)
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
	params.SetTimeout(c.timeout)
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

	return c.fromGeneratedMessage(payload.Message), nil
}

// fromGeneratedMessage converts a generated message to our Message type.
func (c *Client) fromGeneratedMessage(m *models.StromboliInternalHistoryMessage) *Message {
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

// ----------------------------------------------------------------------------
// Auth Methods
// ----------------------------------------------------------------------------

// bearerAuth returns a runtime.ClientAuthInfoWriter for Bearer token auth.
func (c *Client) bearerAuth() runtime.ClientAuthInfoWriter {
	return httptransport.BearerToken(c.token)
}

// SetToken sets the Bearer token for authenticated requests.
//
// This token is used for endpoints that require authentication,
// such as [Client.ValidateToken] and [Client.Logout].
//
// Example:
//
//	tokens, _ := client.GetToken(ctx, "my-client-id")
//	client.SetToken(tokens.AccessToken)
//
//	// Now authenticated endpoints will work
//	validation, _ := client.ValidateToken(ctx)
func (c *Client) SetToken(token string) {
	c.token = token
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
	params.SetTimeout(c.timeout)
	params.SetRequest(&models.TokenRequest{
		ClientID: &clientID,
	})

	// Execute request (uses bearer auth if token is set, for security)
	resp, err := c.api.Auth.PostAuthToken(params, c.bearerAuth())
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
	params.SetTimeout(c.timeout)
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
	if c.token == "" {
		return nil, newError("UNAUTHORIZED", "no token set, use SetToken() first", 401, nil)
	}

	// Create request parameters
	params := auth.NewGetAuthValidateParams()
	params.SetContext(ctx)
	params.SetTimeout(c.timeout)

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
	if c.token == "" {
		return nil, newError("UNAUTHORIZED", "no token set, use SetToken() first", 401, nil)
	}

	// Create request parameters
	params := auth.NewPostAuthLogoutParams()
	params.SetContext(ctx)
	params.SetTimeout(c.timeout)

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
//	for _, name := range secrets {
//	    fmt.Printf("Available secret: %s\n", name)
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
//	            "GITHUB_TOKEN": secrets[0], // Use first available secret
//	        },
//	    },
//	})
func (c *Client) ListSecrets(ctx context.Context) ([]string, error) {
	// Create request parameters
	params := secrets.NewGetSecretsParams()
	params.SetContext(ctx)
	params.SetTimeout(c.timeout)

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

	return payload.Secrets, nil
}
