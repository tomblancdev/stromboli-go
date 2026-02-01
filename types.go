package stromboli

// ----------------------------------------------------------------------------
// System Types
// ----------------------------------------------------------------------------

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

// ----------------------------------------------------------------------------
// Execution Types
// ----------------------------------------------------------------------------

// RunRequest represents a request to execute Claude in an isolated container.
//
// At minimum, you must provide a Prompt. All other fields are optional
// and provide fine-grained control over Claude's execution environment.
//
// Basic usage:
//
//	result, err := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt: "Hello, Claude!",
//	})
//
// With options:
//
//	result, err := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt:  "Review this code",
//	    Workdir: "/workspace",
//	    Claude: &stromboli.ClaudeOptions{
//	        Model:       stromboli.ModelHaiku,
//	        MaxBudgetUSD: 1.0,
//	    },
//	    Podman: &stromboli.PodmanOptions{
//	        Memory:  "1g",
//	        Timeout: "5m",
//	    },
//	})
type RunRequest struct {
	// Prompt is the message to send to Claude. Required.
	Prompt string `json:"prompt"`

	// Workdir is the working directory inside the container.
	// Use Podman.Volumes to mount host paths into the container.
	// Example: "/workspace"
	Workdir string `json:"workdir,omitempty"`

	// WebhookURL is called when an async job completes.
	// Only used with [Client.RunAsync].
	// Example: "https://example.com/webhook"
	WebhookURL string `json:"webhook_url,omitempty"`

	// Claude contains Claude-specific configuration options.
	// See [ClaudeOptions] for available settings.
	Claude *ClaudeOptions `json:"claude,omitempty"`

	// Podman contains container configuration options.
	// See [PodmanOptions] for available settings.
	Podman *PodmanOptions `json:"podman,omitempty"`
}

// ClaudeOptions configures Claude's behavior during execution.
//
// All fields are optional. Use these to customize the model,
// set permissions, configure tools, and more.
//
// Example:
//
//	&stromboli.ClaudeOptions{
//	    Model:                      stromboli.ModelSonnet,
//	    MaxBudgetUSD:               5.0,
//	    AllowedTools:               []string{"Read", "Bash(git:*)"},
//	    DangerouslySkipPermissions: true,
//	}
type ClaudeOptions struct {
	// Model specifies the Claude model to use.
	// Use the Model* constants: ModelHaiku, ModelSonnet, ModelOpus.
	// Default: server-configured default (usually sonnet).
	Model string `json:"model,omitempty"`

	// SessionID enables conversation continuation.
	// Pass a previous response's SessionID to continue the conversation.
	// Example: "550e8400-e29b-41d4-a716-446655440000"
	SessionID string `json:"session_id,omitempty"`

	// Resume continues an existing session (requires SessionID).
	Resume bool `json:"resume,omitempty"`

	// MaxBudgetUSD limits the API cost for this execution.
	// Example: 5.0 means max $5 USD.
	MaxBudgetUSD float64 `json:"max_budget_usd,omitempty"`

	// SystemPrompt replaces the default system prompt entirely.
	// Use AppendSystemPrompt to add to it instead.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// AppendSystemPrompt adds to the default system prompt.
	// Example: "Focus on security best practices"
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"`

	// AllowedTools lists tools Claude can use.
	// Supports patterns like "Bash(git:*)" for git commands only.
	// Example: []string{"Read", "Bash(git:*)", "Edit"}
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// DisallowedTools lists tools Claude cannot use.
	// Example: []string{"Write", "Bash"}
	DisallowedTools []string `json:"disallowed_tools,omitempty"`

	// DangerouslySkipPermissions bypasses all permission checks.
	// Only use in fully sandboxed environments.
	// WARNING: This allows Claude to run any command without confirmation.
	DangerouslySkipPermissions bool `json:"dangerously_skip_permissions,omitempty"`

	// PermissionMode controls how permissions are handled.
	// Values: "default", "acceptEdits", "bypassPermissions", "plan", "dontAsk"
	PermissionMode string `json:"permission_mode,omitempty"`

	// OutputFormat controls the response format.
	// Values: "text", "json", "stream-json"
	OutputFormat string `json:"output_format,omitempty"`

	// Verbose enables detailed logging.
	Verbose bool `json:"verbose,omitempty"`

	// Debug enables debug mode with optional category filter.
	// Example: "api,hooks"
	Debug string `json:"debug,omitempty"`

	// Continue resumes the most recent conversation in workspace.
	// Ignores SessionID if set.
	Continue bool `json:"continue,omitempty"`

	// Agent specifies a predefined agent configuration.
	// Example: "reviewer"
	Agent string `json:"agent,omitempty"`

	// FallbackModel is used when the primary model is overloaded.
	// Example: "haiku"
	FallbackModel string `json:"fallback_model,omitempty"`
}

// PodmanOptions configures the container execution environment.
//
// Use these options to control resource limits, mount volumes,
// and configure container behavior.
//
// Example:
//
//	&stromboli.PodmanOptions{
//	    Memory:  "2g",
//	    Timeout: "10m",
//	    Volumes: []string{"/home/user/project:/workspace:ro"},
//	}
type PodmanOptions struct {
	// Memory limits container memory usage.
	// Examples: "512m", "1g", "2g"
	Memory string `json:"memory,omitempty"`

	// Timeout sets the maximum execution time.
	// Examples: "30s", "5m", "1h"
	Timeout string `json:"timeout,omitempty"`

	// Cpus limits CPU usage.
	// Examples: "0.5" (half a CPU), "2" (two CPUs)
	Cpus string `json:"cpus,omitempty"`

	// CPUShares sets relative CPU weight (default 1024).
	// Lower values = lower priority.
	CPUShares int64 `json:"cpu_shares,omitempty"`

	// Volumes mounts host paths into the container.
	// Format: "host_path:container_path" or "host_path:container_path:options"
	// Options: "ro" (read-only), "rw" (read-write, default)
	// Example: []string{"/data:/data:ro", "/workspace:/workspace"}
	Volumes []string `json:"volumes,omitempty"`

	// Image overrides the container image.
	// Must match server-configured allowed patterns.
	// Example: "python:3.12"
	Image string `json:"image,omitempty"`

	// SecretsEnv injects Podman secrets as environment variables.
	// Key: environment variable name, Value: Podman secret name.
	// The secret must exist (created via `podman secret create`).
	// Example: map[string]string{"GH_TOKEN": "github-token"}
	SecretsEnv map[string]string `json:"secrets_env,omitempty"`
}

// RunResponse represents the result of a synchronous Claude execution.
//
// Check Status to determine if execution succeeded:
//
//	result, err := client.Run(ctx, req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if result.IsSuccess() {
//	    fmt.Println(result.Output)
//	} else {
//	    fmt.Printf("Execution failed: %s\n", result.Error)
//	}
type RunResponse struct {
	// ID is the unique execution identifier.
	// Example: "run-abc123def456"
	ID string `json:"id"`

	// Status indicates execution result.
	// Values: "completed" (success) or "error" (failure).
	Status string `json:"status"`

	// Output contains Claude's response when Status is "completed".
	Output string `json:"output,omitempty"`

	// Error contains the error message when Status is "error".
	Error string `json:"error,omitempty"`

	// SessionID can be used to continue this conversation.
	// Pass this to RunRequest.Claude.SessionID for follow-up requests.
	SessionID string `json:"session_id,omitempty"`
}

// IsSuccess returns true if the execution completed successfully.
func (r *RunResponse) IsSuccess() bool {
	return r.Status == "completed"
}

// AsyncRunResponse represents the result of starting an async execution.
//
// Use the JobID to poll for completion with [Client.GetJob]:
//
//	async, err := client.RunAsync(ctx, req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Poll for completion
//	for {
//	    job, _ := client.GetJob(ctx, async.JobID)
//	    if job.Status == "completed" {
//	        fmt.Println(job.Output)
//	        break
//	    }
//	    time.Sleep(time.Second)
//	}
type AsyncRunResponse struct {
	// JobID is the unique identifier for the async job.
	// Use this with [Client.GetJob] to check status.
	// Example: "job-abc123def456"
	JobID string `json:"job_id"`
}

// ----------------------------------------------------------------------------
// Job Types
// ----------------------------------------------------------------------------

// Job represents the status and result of an async job.
//
// Use [Client.GetJob] to retrieve job status, or [Client.ListJobs] to
// list all jobs:
//
//	job, err := client.GetJob(ctx, "job-abc123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	switch job.Status {
//	case stromboli.JobStatusCompleted:
//	    fmt.Println(job.Output)
//	case stromboli.JobStatusRunning:
//	    fmt.Println("Still running...")
//	case stromboli.JobStatusFailed:
//	    fmt.Printf("Failed: %s\n", job.Error)
//	}
type Job struct {
	// ID is the unique job identifier.
	// Example: "job-abc123def456"
	ID string `json:"id"`

	// Status indicates the current job state.
	// Values: "pending", "running", "completed", "failed", "cancelled"
	Status string `json:"status"`

	// Output contains Claude's response when Status is "completed".
	Output string `json:"output,omitempty"`

	// Error contains the error message when Status is "failed".
	Error string `json:"error,omitempty"`

	// SessionID can be used to continue this conversation.
	// Pass this to RunRequest.Claude.SessionID for follow-up requests.
	SessionID string `json:"session_id,omitempty"`

	// CreatedAt is when the job was created (RFC3339 format).
	// Example: "2024-01-15T10:30:00Z"
	CreatedAt string `json:"created_at,omitempty"`

	// UpdatedAt is when the job was last updated (RFC3339 format).
	// Example: "2024-01-15T10:31:00Z"
	UpdatedAt string `json:"updated_at,omitempty"`

	// CrashInfo contains crash details if the job crashed.
	CrashInfo *CrashInfo `json:"crash_info,omitempty"`
}

// IsCompleted returns true if the job completed successfully.
func (j *Job) IsCompleted() bool {
	return j.Status == JobStatusCompleted
}

// IsRunning returns true if the job is still running.
func (j *Job) IsRunning() bool {
	return j.Status == JobStatusRunning || j.Status == JobStatusPending
}

// IsFailed returns true if the job failed.
func (j *Job) IsFailed() bool {
	return j.Status == JobStatusFailed
}

// IsCancelled returns true if the job was cancelled.
func (j *Job) IsCancelled() bool {
	return j.Status == JobStatusCancelled
}

// CrashInfo contains details about a job crash.
//
// This is populated when a job terminates unexpectedly due to
// container issues, OOM errors, or other infrastructure problems.
type CrashInfo struct {
	// Reason is a human-readable description of why the job crashed.
	// Example: "Container OOM killed", "Timeout exceeded"
	Reason string `json:"reason,omitempty"`

	// ExitCode is the container exit code (if available).
	// Common values: 137 (OOM), 143 (SIGTERM), 1 (general error)
	ExitCode int64 `json:"exit_code,omitempty"`

	// PartialOutput contains any output captured before the crash.
	// This can help debug what the job was doing when it crashed.
	PartialOutput string `json:"partial_output,omitempty"`
}

// ----------------------------------------------------------------------------
// Session Types
// ----------------------------------------------------------------------------

// GetMessagesOptions configures the pagination for [Client.GetMessages].
//
// Example:
//
//	messages, _ := client.GetMessages(ctx, "sess-abc123", &stromboli.GetMessagesOptions{
//	    Limit:  50,
//	    Offset: 100,
//	})
type GetMessagesOptions struct {
	// Limit is the maximum number of messages to return (default: 50, max: 200).
	Limit int64 `json:"limit,omitempty"`

	// Offset is the number of messages to skip (for pagination).
	Offset int64 `json:"offset,omitempty"`
}

// MessagesResponse represents a paginated list of session messages.
//
// Use [Client.GetMessages] to retrieve messages from a session:
//
//	resp, _ := client.GetMessages(ctx, "sess-abc123", nil)
//	for _, msg := range resp.Messages {
//	    fmt.Printf("[%s] %s\n", msg.Role, msg.UUID)
//	}
//
//	if resp.HasMore {
//	    // Fetch more messages...
//	}
type MessagesResponse struct {
	// Messages is the list of messages in this page.
	Messages []*Message `json:"messages"`

	// Total is the total number of messages in the session.
	Total int64 `json:"total"`

	// Limit is the maximum messages per page (requested or default).
	Limit int64 `json:"limit"`

	// Offset is the number of messages skipped.
	Offset int64 `json:"offset"`

	// HasMore indicates if there are more messages to fetch.
	HasMore bool `json:"has_more"`
}

// Message represents a single message from session history.
//
// Messages can be user prompts, assistant responses, or tool interactions.
// Use [Client.GetMessages] to list messages or [Client.GetMessage] to get
// a specific message by UUID.
type Message struct {
	// UUID is the unique message identifier.
	// Example: "92242819-b7d1-48d4-b023-6134c3e9f63a"
	UUID string `json:"uuid,omitempty"`

	// Type indicates the message type.
	// Values: "user", "assistant", "queue-operation"
	Type string `json:"type,omitempty"`

	// ParentUUID is the parent message UUID for threading.
	ParentUUID string `json:"parent_uuid,omitempty"`

	// SessionID is the session this message belongs to.
	SessionID string `json:"session_id,omitempty"`

	// Cwd is the working directory at time of message.
	// Example: "/workspace"
	Cwd string `json:"cwd,omitempty"`

	// GitBranch is the git branch at time of message.
	// Example: "main"
	GitBranch string `json:"git_branch,omitempty"`

	// PermissionMode is the permission mode active for this message.
	// Example: "bypassPermissions"
	PermissionMode string `json:"permission_mode,omitempty"`

	// Timestamp is when the message was created (RFC3339 format).
	Timestamp string `json:"timestamp,omitempty"`

	// Version is the Claude Code version that created this message.
	Version string `json:"version,omitempty"`

	// Content contains the message content (text, tool calls, etc.).
	// The structure varies by message type.
	Content interface{} `json:"content,omitempty"`

	// ToolResult contains tool use results (for tool_result messages).
	ToolResult interface{} `json:"tool_result,omitempty"`
}

// ----------------------------------------------------------------------------
// Constants
// ----------------------------------------------------------------------------

// Model constants for Claude model selection.
//
// Use these with [ClaudeOptions.Model]:
//
//	&stromboli.ClaudeOptions{
//	    Model: stromboli.ModelHaiku,
//	}
const (
	// ModelHaiku is the fastest and most cost-effective model.
	// Best for simple tasks, quick responses, and high-volume use cases.
	ModelHaiku = "haiku"

	// ModelSonnet is the balanced model for most use cases.
	// Good balance of speed, capability, and cost.
	ModelSonnet = "sonnet"

	// ModelOpus is the most capable model.
	// Best for complex reasoning, nuanced tasks, and highest quality output.
	ModelOpus = "opus"
)

// RunStatus constants for execution results.
const (
	// RunStatusCompleted indicates successful execution.
	RunStatusCompleted = "completed"

	// RunStatusError indicates execution failed.
	RunStatusError = "error"
)

// HealthStatus constants for convenience.
const (
	// StatusOK indicates the service or component is healthy.
	StatusOK = "ok"

	// StatusError indicates the service or component has an error.
	StatusError = "error"
)

// JobStatus constants for async job states.
//
// Use these with [Job.Status]:
//
//	if job.Status == stromboli.JobStatusCompleted {
//	    fmt.Println(job.Output)
//	}
const (
	// JobStatusPending indicates the job is queued but not yet started.
	JobStatusPending = "pending"

	// JobStatusRunning indicates the job is currently executing.
	JobStatusRunning = "running"

	// JobStatusCompleted indicates the job completed successfully.
	JobStatusCompleted = "completed"

	// JobStatusFailed indicates the job failed with an error.
	JobStatusFailed = "failed"

	// JobStatusCancelled indicates the job was cancelled.
	JobStatusCancelled = "cancelled"
)

// ----------------------------------------------------------------------------
// Auth Types
// ----------------------------------------------------------------------------

// TokenResponse represents JWT tokens returned by authentication endpoints.
//
// Use [Client.GetToken] to obtain tokens:
//
//	tokens, err := client.GetToken(ctx, "my-client-id")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Access token: %s\n", tokens.AccessToken)
type TokenResponse struct {
	// AccessToken is the JWT access token for API authentication.
	// Use with [WithToken] option or pass to authenticated endpoints.
	AccessToken string `json:"access_token"`

	// RefreshToken is used to obtain new access tokens.
	// Use with [Client.RefreshToken] when the access token expires.
	RefreshToken string `json:"refresh_token"`

	// ExpiresIn is the access token lifetime in seconds.
	// Example: 3600 (1 hour)
	ExpiresIn int64 `json:"expires_in"`

	// TokenType is the token type, typically "Bearer".
	TokenType string `json:"token_type"`
}

// TokenValidation represents the result of validating a JWT token.
//
// Use [Client.ValidateToken] to validate the current token:
//
//	validation, err := client.ValidateToken(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if validation.Valid {
//	    fmt.Printf("Token valid until: %d\n", validation.ExpiresAt)
//	}
type TokenValidation struct {
	// Valid indicates whether the token is valid.
	Valid bool `json:"valid"`

	// Subject is the token subject (typically client ID).
	Subject string `json:"subject"`

	// ExpiresAt is the token expiration time as Unix timestamp.
	ExpiresAt int64 `json:"expires_at"`
}

// LogoutResponse represents the result of invalidating a token.
//
// Use [Client.Logout] to invalidate the current token:
//
//	result, err := client.Logout(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if result.Success {
//	    fmt.Println("Logged out successfully")
//	}
type LogoutResponse struct {
	// Success indicates whether the logout was successful.
	Success bool `json:"success"`

	// Message provides additional context about the logout.
	Message string `json:"message"`
}
