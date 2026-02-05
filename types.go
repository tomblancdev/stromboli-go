package stromboli

import "time"

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
	return h.Status == StatusOK
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
	return c.Status == StatusOK
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
	Model Model `json:"model,omitempty"`

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

	// JSONSchema specifies a JSON Schema for structured output validation.
	// When set, Claude's output MUST conform to this schema.
	// Requires OutputFormat to be "json" for best results.
	//
	// If the output does not match the schema, the API may return an error
	// or Claude may retry to produce conforming output (behavior depends on
	// the Stromboli server configuration).
	//
	// Example:
	//
	//	&stromboli.ClaudeOptions{
	//	    OutputFormat: "json",
	//	    JSONSchema: `{
	//	        "type": "object",
	//	        "required": ["summary", "score"],
	//	        "properties": {
	//	            "summary": {"type": "string"},
	//	            "score": {"type": "integer", "minimum": 0, "maximum": 100}
	//	        }
	//	    }`,
	//	}
	//
	// See: https://json-schema.org/specification
	JSONSchema string `json:"json_schema,omitempty"`

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

	// AddDirs specifies additional directories for tool access.
	// Example: []string{"/home/user/shared", "/data"}
	AddDirs []string `json:"add_dirs,omitempty"`

	// Agents specifies custom agents definition (JSON object).
	// Example: map[string]interface{}{"reviewer": ...}
	Agents map[string]interface{} `json:"agents,omitempty"`

	// AllowDangerouslySkipPermissions enables bypass as an option without enabling by default.
	AllowDangerouslySkipPermissions bool `json:"allow_dangerously_skip_permissions,omitempty"`

	// Betas specifies beta headers for API requests.
	// Example: []string{"interleaved-thinking-2025-05-14"}
	Betas []string `json:"betas,omitempty"`

	// DisableSlashCommands disables all slash commands/skills.
	DisableSlashCommands bool `json:"disable_slash_commands,omitempty"`

	// Files specifies file resources in format: file_id:path.
	// Example: []string{"abc123:/workspace/file.txt"}
	Files []string `json:"files,omitempty"`

	// ForkSession creates a new session ID when resuming.
	ForkSession bool `json:"fork_session,omitempty"`

	// IncludePartialMessages includes partial message chunks (stream-json only).
	IncludePartialMessages bool `json:"include_partial_messages,omitempty"`

	// InputFormat specifies the input format: text, stream-json.
	// Example: "text"
	InputFormat string `json:"input_format,omitempty"`

	// McpConfigs specifies MCP server config files or JSON strings.
	// Example: []string{"/path/to/mcp.json"}
	McpConfigs []string `json:"mcp_configs,omitempty"`

	// NoPersistence prevents saving session to disk.
	NoPersistence bool `json:"no_persistence,omitempty"`

	// PluginDirs specifies plugin directories.
	// Example: []string{"/home/user/.claude/plugins"}
	PluginDirs []string `json:"plugin_dirs,omitempty"`

	// ReplayUserMessages re-emits user messages on stdout.
	ReplayUserMessages bool `json:"replay_user_messages,omitempty"`

	// SettingSources specifies setting sources to load: user, project, local.
	// Example: []string{"user", "project"}
	SettingSources []string `json:"setting_sources,omitempty"`

	// Settings specifies path to settings JSON file or JSON string.
	// Example: "/path/to/settings.json"
	Settings string `json:"settings,omitempty"`

	// StrictMcpConfig only uses MCP servers from mcp_configs.
	StrictMcpConfig bool `json:"strict_mcp_config,omitempty"`

	// Tools specifies built-in tools ("", "default", or specific names).
	// Example: []string{"Bash", "Read", "Edit"}
	Tools []string `json:"tools,omitempty"`
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

	// Lifecycle configures commands to run at specific container lifecycle stages.
	// See [LifecycleHooks] for available hooks.
	Lifecycle *LifecycleHooks `json:"lifecycle,omitempty"`

	// Environment specifies a compose-based multi-service environment.
	// When set, the agent runs inside the specified service of the compose stack.
	// See [EnvironmentConfig] for configuration options.
	Environment *EnvironmentConfig `json:"environment,omitempty"`
}

// LifecycleHooks configures commands to run at specific container lifecycle stages.
//
// Use these hooks to set up the container environment before Claude starts,
// such as installing dependencies, starting background services, etc.
//
// Example:
//
//	&stromboli.LifecycleHooks{
//	    OnCreateCommand: []string{"pip install -r requirements.txt"},
//	    PostStart:       []string{"redis-server --daemonize yes"},
//	    HooksTimeout:    "5m",
//	}
type LifecycleHooks struct {
	// OnCreateCommand runs after container creation, before Claude starts (first run only).
	// Commands are executed sequentially via "podman exec".
	// Example: []string{"pip install -r requirements.txt"}
	OnCreateCommand []string `json:"on_create_command,omitempty"`

	// PostCreate runs after OnCreateCommand completes (first run only).
	// Commands are executed sequentially via "podman exec".
	// Example: []string{"npm run setup"}
	PostCreate []string `json:"post_create,omitempty"`

	// PostStart runs after container starts (every run, including continues).
	// Commands are executed sequentially via "podman exec".
	// Example: []string{"redis-server --daemonize yes"}
	PostStart []string `json:"post_start,omitempty"`

	// HooksTimeout is the maximum duration for all hooks combined.
	// If not specified, hooks run with the container's timeout.
	// Examples: "5m", "30s"
	HooksTimeout string `json:"hooks_timeout,omitempty"`
}

// EnvironmentConfig specifies a compose-based multi-service environment.
//
// When set, the agent runs inside the specified service of a Docker Compose
// stack instead of a standalone container. This allows running Claude
// in complex multi-container environments.
//
// Example:
//
//	&stromboli.EnvironmentConfig{
//	    Type:    "compose",
//	    Path:    "/home/user/project/docker-compose.yml",
//	    Service: "dev",
//	}
type EnvironmentConfig struct {
	// Type of environment: "" (default single container) or "compose".
	// Example: "compose"
	Type string `json:"type,omitempty"`

	// Path to compose file (required when Type="compose").
	// Must be an absolute path ending in .yml or .yaml.
	// Example: "/home/user/project/docker-compose.yml"
	Path string `json:"path,omitempty"`

	// Service name where Claude will run (required when Type="compose").
	// Example: "dev"
	Service string `json:"service,omitempty"`

	// BuildTimeout is the optional build timeout override for compose.
	// If not specified, uses server default (10m).
	// Examples: "15m", "30m"
	BuildTimeout string `json:"build_timeout,omitempty"`
}

// RunResponse represents the result of a synchronous Claude execution.
//
// Important: A nil error from [Client.Run] means the API call succeeded,
// not necessarily that Claude execution succeeded. Always check Status
// and Error fields to determine if the execution completed successfully.
//
// Check Status to determine if execution succeeded:
//
//	result, err := client.Run(ctx, req)
//	if err != nil {
//	    log.Fatal(err) // API call failed
//	}
//	if result.IsSuccess() {
//	    fmt.Println(result.Output)
//	} else {
//	    // Execution failed - check result.Error for details
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
	return r.Status == RunStatusCompleted
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

// IsPending returns true if the job is pending (queued but not yet started).
func (j *Job) IsPending() bool {
	return j.Status == JobStatusPending
}

// CreatedAtTime parses CreatedAt as time.Time.
// Returns zero time if CreatedAt is empty or parsing fails.
func (j *Job) CreatedAtTime() time.Time {
	if j.CreatedAt == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, j.CreatedAt)
	return t
}

// UpdatedAtTime parses UpdatedAt as time.Time.
// Returns zero time if UpdatedAt is empty or parsing fails.
func (j *Job) UpdatedAtTime() time.Time {
	if j.UpdatedAt == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, j.UpdatedAt)
	return t
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

	// Signal is the signal that killed the process (if applicable).
	// Examples: "SIGSEGV", "SIGKILL", "SIGTERM"
	Signal string `json:"signal,omitempty"`

	// TaskCompleted indicates whether the task appeared to complete before crashing.
	TaskCompleted bool `json:"task_completed,omitempty"`
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
	// The structure varies by message type:
	//   - For "user" messages: string or []ContentBlock
	//   - For "assistant" messages: []ContentBlock with text and tool_use
	//
	// Use type assertions or json.Marshal/Unmarshal to work with this field.
	//
	// Example:
	//
	//	// Check if content is a simple string
	//	if text, ok := msg.Content.(string); ok {
	//	    fmt.Println(text)
	//	}
	//
	//	// For complex content, marshal and unmarshal
	//	data, _ := json.Marshal(msg.Content)
	//	var blocks []map[string]interface{}
	//	json.Unmarshal(data, &blocks)
	Content interface{} `json:"content,omitempty"`

	// ToolResult contains tool use results (for tool_result messages).
	// The structure is typically:
	//   - ToolUseID: string - The ID of the tool use this result responds to
	//   - Content: string or []ContentBlock - The result data
	//   - IsError: bool - Whether this result represents an error
	//
	// Use type assertions or json.Marshal/Unmarshal to work with this field.
	ToolResult interface{} `json:"tool_result,omitempty"`
}

// TimestampTime parses Timestamp as time.Time.
// Returns zero time if Timestamp is empty or parsing fails.
func (m *Message) TimestampTime() time.Time {
	if m.Timestamp == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, m.Timestamp)
	return t
}

// ContentAsString returns the content as a string if it is a simple string message.
// Returns empty string and false if content is not a string.
//
// Example:
//
//	if text, ok := msg.ContentAsString(); ok {
//	    fmt.Println(text)
//	}
func (m *Message) ContentAsString() (string, bool) {
	s, ok := m.Content.(string)
	return s, ok
}

// ContentAsBlocks returns the content as a slice of maps if it contains content blocks.
// Returns nil and false if content is not in block format.
//
// For more precise typing, use json.Marshal/Unmarshal:
//
//	data, _ := json.Marshal(msg.Content)
//	var blocks []YourBlockType
//	json.Unmarshal(data, &blocks)
func (m *Message) ContentAsBlocks() ([]map[string]interface{}, bool) {
	blocks, ok := m.Content.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]map[string]interface{}, 0, len(blocks))
	for _, b := range blocks {
		if block, ok := b.(map[string]interface{}); ok {
			result = append(result, block)
		}
	}
	return result, len(result) > 0
}

// ----------------------------------------------------------------------------
// Secrets Types
// ----------------------------------------------------------------------------

// Secret represents a Podman secret's metadata.
//
// Secrets are used to securely pass sensitive data (API keys, tokens, etc.)
// to containers without exposing them in environment variables or command lines.
//
// Use [Client.CreateSecret] to create a new secret:
//
//	err := client.CreateSecret(ctx, &stromboli.CreateSecretRequest{
//	    Name:  "github-token",
//	    Value: "ghp_xxxx...",
//	})
type Secret struct {
	// ID is the unique identifier of the secret.
	// Example: "abc123def456"
	ID string `json:"id,omitempty"`

	// Name is the secret name used to reference it.
	// Example: "github-token"
	Name string `json:"name"`

	// CreatedAt is when the secret was created (RFC3339 format).
	// Example: "2024-01-15T10:30:00Z"
	CreatedAt string `json:"created_at,omitempty"`
}

// CreatedAtTime parses CreatedAt as time.Time.
// Returns zero time if CreatedAt is empty or parsing fails.
func (s *Secret) CreatedAtTime() time.Time {
	if s.CreatedAt == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s.CreatedAt)
	return t
}

// CreateSecretRequest represents a request to create a new Podman secret.
//
// Use with [Client.CreateSecret]:
//
//	err := client.CreateSecret(ctx, &stromboli.CreateSecretRequest{
//	    Name:  "github-token",
//	    Value: "ghp_xxxx...",
//	})
type CreateSecretRequest struct {
	// Name is the secret name (required).
	// Must be unique among existing secrets.
	// Example: "github-token"
	Name string `json:"name"`

	// Value is the secret data (required).
	// This value is stored securely and never returned by the API.
	// Example: "ghp_xxxx..."
	Value string `json:"value"`
}

// ----------------------------------------------------------------------------
// Images Types
// ----------------------------------------------------------------------------

// Image represents a local container image with compatibility information.
//
// Use [Client.ListImages] to list all available images:
//
//	images, err := client.ListImages(ctx)
//	for _, img := range images {
//	    fmt.Printf("%s:%s (rank %d)\n", img.Repository, img.Tag, img.CompatibilityRank)
//	}
type Image struct {
	// ID is the image ID (usually sha256:...).
	// Example: "sha256:abc123def456"
	ID string `json:"id,omitempty"`

	// Repository is the image repository name.
	// Example: "python"
	Repository string `json:"repository,omitempty"`

	// Tag is the image tag.
	// Example: "3.12-slim"
	Tag string `json:"tag,omitempty"`

	// Size is the image size in bytes.
	// Example: 125000000
	Size int64 `json:"size,omitempty"`

	// Created is when the image was created (RFC3339 format).
	// Example: "2024-01-15T10:30:00Z"
	Created string `json:"created,omitempty"`

	// Description is a human-readable description of the image.
	// Example: "Python development image"
	Description string `json:"description,omitempty"`

	// Compatible indicates if the image is compatible with Stromboli.
	// Images with glibc are compatible; Alpine/musl images are not.
	Compatible bool `json:"compatible,omitempty"`

	// CompatibilityRank indicates the image's compatibility level.
	// 1-2: Verified compatible, 3: Standard glibc, 4: Incompatible (Alpine/musl)
	CompatibilityRank int64 `json:"compatibility_rank,omitempty"`

	// HasClaudeCLI indicates if the image has Claude CLI pre-installed.
	HasClaudeCLI bool `json:"has_claude_cli,omitempty"`

	// Tools lists tools available in the image.
	// Example: []string{"python", "pip", "git"}
	Tools []string `json:"tools,omitempty"`
}

// CreatedTime parses Created as time.Time.
// Returns zero time if Created is empty or parsing fails.
func (i *Image) CreatedTime() time.Time {
	if i.Created == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, i.Created)
	return t
}

// ImageSearchResult represents a search result from a container registry.
//
// Use [Client.SearchImages] to search registries:
//
//	results, err := client.SearchImages(ctx, &stromboli.SearchImagesOptions{
//	    Query: "python",
//	    Limit: 10,
//	})
//	for _, r := range results {
//	    fmt.Printf("%s: %s (stars: %d)\n", r.Name, r.Description, r.Stars)
//	}
type ImageSearchResult struct {
	// Name is the image name.
	// Example: "python"
	Name string `json:"name,omitempty"`

	// Description is the image description from the registry.
	// Example: "Python is an interpreted programming language"
	Description string `json:"description,omitempty"`

	// Stars is the number of stars on the registry.
	// Example: 8500
	Stars int64 `json:"stars,omitempty"`

	// Official indicates if this is an official image.
	Official bool `json:"official,omitempty"`

	// Automated indicates if this image is automatically built.
	Automated bool `json:"automated,omitempty"`

	// Index is the registry index (e.g., "docker.io").
	// Example: "docker.io"
	Index string `json:"index,omitempty"`
}

// SearchImagesOptions configures an image search request.
//
// Example:
//
//	results, err := client.SearchImages(ctx, &stromboli.SearchImagesOptions{
//	    Query:   "python",
//	    Limit:   25,
//	    NoTrunc: true,
//	})
type SearchImagesOptions struct {
	// Query is the search term (required).
	// Example: "python"
	Query string

	// Limit is the maximum number of results to return.
	// Default varies by registry.
	Limit int64

	// NoTrunc disables truncation of output.
	NoTrunc bool
}

// PullImageRequest represents a request to pull a container image.
//
// Use with [Client.PullImage]:
//
//	result, err := client.PullImage(ctx, &stromboli.PullImageRequest{
//	    Image:    "python:3.12-slim",
//	    Platform: "linux/amd64",
//	})
type PullImageRequest struct {
	// Image is the image reference to pull (required).
	// Example: "python:3.12-slim"
	Image string `json:"image"`

	// Platform specifies the platform for multi-arch images.
	// Example: "linux/amd64", "linux/arm64"
	Platform string `json:"platform,omitempty"`

	// Quiet suppresses pull progress output.
	Quiet bool `json:"quiet,omitempty"`
}

// PullImageResponse represents the result of an image pull operation.
type PullImageResponse struct {
	// Success indicates if the pull was successful.
	Success bool `json:"success,omitempty"`

	// Image is the pulled image reference.
	// Example: "python:3.12-slim"
	Image string `json:"image,omitempty"`

	// ImageID is the pulled image's ID.
	// Example: "sha256:abc123def456"
	ImageID string `json:"image_id,omitempty"`
}

// ----------------------------------------------------------------------------
// Constants
// ----------------------------------------------------------------------------

// Model represents a Claude model identifier.
//
// The SDK provides constants for common models (ModelHaiku, ModelSonnet, ModelOpus).
// For newer models not yet added to the SDK, you can cast any string to Model:
//
//	customModel := stromboli.Model("claude-3-5-sonnet-20241022")
//
// Model values are passed directly to the API, so you can use any model
// identifier supported by the Stromboli server.
type Model string

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
	ModelHaiku Model = "haiku"

	// ModelSonnet is the balanced model for most use cases.
	// Good balance of speed, capability, and cost.
	ModelSonnet Model = "sonnet"

	// ModelOpus is the most capable model.
	// Best for complex reasoning, nuanced tasks, and highest quality output.
	ModelOpus Model = "opus"
)

// String returns the string representation of the Model.
func (m Model) String() string {
	return string(m)
}

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
