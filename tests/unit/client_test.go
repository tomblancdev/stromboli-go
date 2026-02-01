package unit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// mustEncode encodes v as JSON and writes it to w.
// Panics on error - safe in tests since errors indicate test bugs.
func mustEncode(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic("failed to encode response: " + err.Error())
	}
}

// mustDecode decodes JSON from r.Body into v.
// Panics on error - safe in tests since errors indicate test bugs.
func mustDecode(r *http.Request, v interface{}) {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		panic("failed to decode request: " + err.Error())
	}
}

// TestHealth_Success tests the Health method with a successful response.
//
// It verifies that:
//   - The client correctly calls the /health endpoint
//   - The response is properly parsed into HealthResponse
//   - Helper methods like IsHealthy() work correctly
func TestHealth_Success(t *testing.T) {
	// Arrange: Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"name":    "stromboli",
			"status":  "ok",
			"version": "0.3.0-alpha",
			"components": []map[string]interface{}{
				{"name": "podman", "status": "ok", "error": ""},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act: Create client and call Health
	client := stromboli.NewClient(server.URL)
	health, err := client.Health(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "stromboli", health.Name)
	assert.Equal(t, "ok", health.Status)
	assert.Equal(t, "0.3.0-alpha", health.Version)
	assert.True(t, health.IsHealthy())
	assert.Len(t, health.Components, 1)
	assert.Equal(t, "podman", health.Components[0].Name)
	assert.True(t, health.Components[0].IsHealthy())
}

// TestHealth_Unhealthy tests the Health method when the API reports unhealthy status.
func TestHealth_Unhealthy(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"name":    "stromboli",
			"status":  "error",
			"version": "0.3.0-alpha",
			"components": []map[string]interface{}{
				{"name": "podman", "status": "error", "error": "podman not found"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	health, err := client.Health(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "error", health.Status)
	assert.False(t, health.IsHealthy())
	assert.False(t, health.Components[0].IsHealthy())
	assert.Equal(t, "podman not found", health.Components[0].Error)
}

// TestHealth_ServerError tests error handling when the server returns 500.
func TestHealth_ServerError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		mustEncode(w, map[string]string{"error": "internal error"})
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	health, err := client.Health(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, health)

	// Verify error type
	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "INTERNAL", apiErr.Code)
}

// TestHealth_ContextCancellation tests that context cancellation is handled correctly.
func TestHealth_ContextCancellation(t *testing.T) {
	// Arrange: Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if context was cancelled
		<-r.Context().Done()
	}))
	defer server.Close()

	// Act: Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := stromboli.NewClient(server.URL)
	health, err := client.Health(ctx)

	// Assert
	require.Error(t, err)
	assert.Nil(t, health)
}

// TestClaudeStatus_Configured tests ClaudeStatus when Claude is configured.
func TestClaudeStatus_Configured(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/claude/status", r.URL.Path)

		resp := map[string]interface{}{
			"configured": true,
			"message":    "Claude is configured",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	status, err := client.ClaudeStatus(context.Background())

	// Assert
	require.NoError(t, err)
	assert.True(t, status.Configured)
	assert.Equal(t, "Claude is configured", status.Message)
}

// TestClaudeStatus_NotConfigured tests ClaudeStatus when Claude is not configured.
func TestClaudeStatus_NotConfigured(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"configured": false,
			"message":    "ANTHROPIC_API_KEY not set",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	status, err := client.ClaudeStatus(context.Background())

	// Assert
	require.NoError(t, err)
	assert.False(t, status.Configured)
	assert.Contains(t, status.Message, "ANTHROPIC_API_KEY")
}

// TestNewClient_Options tests that client options are applied correctly.
func TestNewClient_Options(t *testing.T) {
	// This test verifies options don't panic and the client is created.
	// Actual behavior testing would require inspecting internal state.

	customHTTPClient := &http.Client{}

	client := stromboli.NewClient("http://localhost:8585",
		stromboli.WithTimeout(60),
		stromboli.WithRetries(3),
		stromboli.WithHTTPClient(customHTTPClient),
		stromboli.WithUserAgent("test-agent/1.0"),
	)

	assert.NotNil(t, client)
}

// ----------------------------------------------------------------------------
// Execution Method Tests
// ----------------------------------------------------------------------------

// TestRun_Success tests the Run method with a successful execution.
//
// It verifies that:
//   - The client correctly calls the /run endpoint with POST
//   - The request body contains the prompt
//   - The response is properly parsed into RunResponse
//   - Helper method IsSuccess() works correctly
func TestRun_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/run", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)
		assert.Equal(t, "Hello, Claude!", req["prompt"])

		// Return mock response
		resp := map[string]interface{}{
			"id":         "run-abc123",
			"status":     "completed",
			"output":     "Hello! How can I help you today?",
			"session_id": "sess-xyz789",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Hello, Claude!",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "run-abc123", result.ID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "Hello! How can I help you today?", result.Output)
	assert.Equal(t, "sess-xyz789", result.SessionID)
	assert.True(t, result.IsSuccess())
}

// TestRun_WithOptions tests Run with Claude and Podman options.
func TestRun_WithOptions(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)

		// Verify prompt
		assert.Equal(t, "Review this code", req["prompt"])
		assert.Equal(t, "/workspace", req["workdir"])

		// Verify Claude options
		claude, ok := req["claude"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "sonnet", claude["model"])
		assert.Equal(t, float64(5.0), claude["max_budget_usd"])

		// Verify Podman options
		podman, ok := req["podman"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "2g", podman["memory"])
		assert.Equal(t, "10m", podman["timeout"])

		// Return mock response
		resp := map[string]interface{}{
			"id":     "run-def456",
			"status": "completed",
			"output": "Code review complete",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt:  "Review this code",
		Workdir: "/workspace",
		Claude: &stromboli.ClaudeOptions{
			Model:        stromboli.ModelSonnet,
			MaxBudgetUSD: 5.0,
			AllowedTools: []string{"Read", "Glob"},
		},
		Podman: &stromboli.PodmanOptions{
			Memory:  "2g",
			Timeout: "10m",
		},
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.True(t, result.IsSuccess())
}

// TestRun_EmptyPrompt tests that Run returns an error when prompt is empty.
func TestRun_EmptyPrompt(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "",
	})

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestRun_NilRequest tests that Run returns an error when request is nil.
func TestRun_NilRequest(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	result, err := client.Run(context.Background(), nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestRun_ExecutionError tests Run when Claude execution fails.
func TestRun_ExecutionError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":     "run-err789",
			"status": "error",
			"error":  "Claude execution failed: timeout",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Do something complex",
	})

	// Assert
	require.NoError(t, err) // Request succeeded, but execution failed
	assert.Equal(t, "error", result.Status)
	assert.False(t, result.IsSuccess())
	assert.Contains(t, result.Error, "timeout")
}

// TestRun_ServerError tests Run when the server returns 500.
func TestRun_ServerError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		mustEncode(w, map[string]string{"error": "internal server error"})
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Hello",
	})

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)

	// Verify it's an API error (code varies by how go-swagger handles the response)
	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.NotEmpty(t, apiErr.Code)
	assert.Contains(t, apiErr.Message, "failed")
}

// TestRunAsync_Success tests the RunAsync method with a successful start.
func TestRunAsync_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/run/async", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)
		assert.Equal(t, "Analyze this codebase", req["prompt"])

		// Return mock response
		resp := map[string]interface{}{
			"job_id": "job-abc123",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	result, err := client.RunAsync(context.Background(), &stromboli.RunRequest{
		Prompt: "Analyze this codebase",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "job-abc123", result.JobID)
}

// TestRunAsync_WithWebhook tests RunAsync with a webhook URL.
func TestRunAsync_WithWebhook(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)

		// Verify webhook URL is passed
		assert.Equal(t, "https://example.com/webhook", req["webhook_url"])

		// Return mock response
		resp := map[string]interface{}{
			"job_id": "job-webhook123",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	result, err := client.RunAsync(context.Background(), &stromboli.RunRequest{
		Prompt:     "Long running task",
		WebhookURL: "https://example.com/webhook",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "job-webhook123", result.JobID)
}

// TestRunAsync_EmptyPrompt tests that RunAsync returns an error when prompt is empty.
func TestRunAsync_EmptyPrompt(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	result, err := client.RunAsync(context.Background(), &stromboli.RunRequest{
		Prompt: "",
	})

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestRunAsync_NilRequest tests that RunAsync returns an error when request is nil.
func TestRunAsync_NilRequest(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	result, err := client.RunAsync(context.Background(), nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// ----------------------------------------------------------------------------
// Job Method Tests
// ----------------------------------------------------------------------------

// TestListJobs_Success tests the ListJobs method with multiple jobs.
func TestListJobs_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"jobs": []map[string]interface{}{
				{
					"id":         "job-001",
					"status":     "completed",
					"output":     "Task completed",
					"session_id": "sess-001",
					"created_at": "2024-01-15T10:30:00Z",
				},
				{
					"id":         "job-002",
					"status":     "running",
					"created_at": "2024-01-15T10:35:00Z",
				},
				{
					"id":         "job-003",
					"status":     "pending",
					"created_at": "2024-01-15T10:40:00Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	jobs, err := client.ListJobs(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, jobs, 3)

	// Check first job (completed)
	assert.Equal(t, "job-001", jobs[0].ID)
	assert.Equal(t, "completed", jobs[0].Status)
	assert.Equal(t, "Task completed", jobs[0].Output)
	assert.True(t, jobs[0].IsCompleted())

	// Check second job (running)
	assert.Equal(t, "job-002", jobs[1].ID)
	assert.Equal(t, "running", jobs[1].Status)
	assert.True(t, jobs[1].IsRunning())

	// Check third job (pending)
	assert.Equal(t, "job-003", jobs[2].ID)
	assert.Equal(t, "pending", jobs[2].Status)
	assert.True(t, jobs[2].IsRunning()) // Pending is considered running
}

// TestListJobs_Empty tests ListJobs when no jobs exist.
func TestListJobs_Empty(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"jobs": []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	jobs, err := client.ListJobs(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

// TestGetJob_Success tests the GetJob method with a completed job.
func TestGetJob_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/jobs/job-abc123", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"id":         "job-abc123",
			"status":     "completed",
			"output":     "Hello from Claude!",
			"session_id": "sess-xyz789",
			"created_at": "2024-01-15T10:30:00Z",
			"updated_at": "2024-01-15T10:31:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	job, err := client.GetJob(context.Background(), "job-abc123")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "job-abc123", job.ID)
	assert.Equal(t, "completed", job.Status)
	assert.Equal(t, "Hello from Claude!", job.Output)
	assert.Equal(t, "sess-xyz789", job.SessionID)
	assert.True(t, job.IsCompleted())
	assert.False(t, job.IsRunning())
	assert.False(t, job.IsFailed())
}

// TestGetJob_Failed tests GetJob with a failed job.
func TestGetJob_Failed(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":     "job-failed",
			"status": "failed",
			"error":  "Claude execution timed out",
			"crash_info": map[string]interface{}{
				"reason":         "Timeout exceeded",
				"exit_code":      137,
				"partial_output": "Processing file 1 of 100...",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	job, err := client.GetJob(context.Background(), "job-failed")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "failed", job.Status)
	assert.True(t, job.IsFailed())
	assert.Contains(t, job.Error, "timed out")

	// Check crash info
	require.NotNil(t, job.CrashInfo)
	assert.Equal(t, "Timeout exceeded", job.CrashInfo.Reason)
	assert.Equal(t, int64(137), job.CrashInfo.ExitCode)
	assert.Contains(t, job.CrashInfo.PartialOutput, "Processing")
}

// TestGetJob_NotFound tests GetJob with an invalid job ID.
func TestGetJob_NotFound(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustEncode(w, map[string]string{"error": "job not found"})
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	job, err := client.GetJob(context.Background(), "invalid-id")

	// Assert
	require.Error(t, err)
	assert.Nil(t, job)

	// Verify it's an API error (error code varies by go-swagger error handling)
	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.NotEmpty(t, apiErr.Code)
}

// TestGetJob_EmptyID tests GetJob with an empty job ID.
func TestGetJob_EmptyID(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	job, err := client.GetJob(context.Background(), "")

	// Assert
	require.Error(t, err)
	assert.Nil(t, job)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestCancelJob_Success tests the CancelJob method.
func TestCancelJob_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/jobs/job-cancel123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		// Return success (200 OK)
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, map[string]string{"status": "cancelled"})
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	err := client.CancelJob(context.Background(), "job-cancel123")

	// Assert
	require.NoError(t, err)
}

// TestCancelJob_NotFound tests CancelJob with an invalid job ID.
func TestCancelJob_NotFound(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustEncode(w, map[string]string{"error": "job not found"})
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	err := client.CancelJob(context.Background(), "invalid-id")

	// Assert
	require.Error(t, err)

	// Verify it's an API error (error code varies by go-swagger error handling)
	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.NotEmpty(t, apiErr.Code)
}

// TestCancelJob_EmptyID tests CancelJob with an empty job ID.
func TestCancelJob_EmptyID(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	err := client.CancelJob(context.Background(), "")

	// Assert
	require.Error(t, err)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// ----------------------------------------------------------------------------
// Session Method Tests
// ----------------------------------------------------------------------------

// TestListSessions_Success tests the ListSessions method with multiple sessions.
func TestListSessions_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/sessions", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"sessions": []string{
				"sess-abc123",
				"sess-def456",
				"sess-ghi789",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	sessions, err := client.ListSessions(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, sessions, 3)
	assert.Equal(t, "sess-abc123", sessions[0])
	assert.Equal(t, "sess-def456", sessions[1])
	assert.Equal(t, "sess-ghi789", sessions[2])
}

// TestListSessions_Empty tests ListSessions when no sessions exist.
func TestListSessions_Empty(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"sessions": []string{},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	sessions, err := client.ListSessions(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

// TestDestroySession_Success tests the DestroySession method.
func TestDestroySession_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/sessions/sess-abc123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		// Return success
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, map[string]string{"status": "destroyed"})
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	err := client.DestroySession(context.Background(), "sess-abc123")

	// Assert
	require.NoError(t, err)
}

// TestDestroySession_EmptyID tests DestroySession with an empty session ID.
func TestDestroySession_EmptyID(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	err := client.DestroySession(context.Background(), "")

	// Assert
	require.Error(t, err)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestGetMessages_Success tests the GetMessages method.
func TestGetMessages_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/sessions/sess-abc123/messages", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"uuid":       "msg-001",
					"type":       "user",
					"session_id": "sess-abc123",
					"timestamp":  "2024-01-15T10:30:00Z",
				},
				{
					"uuid":       "msg-002",
					"type":       "assistant",
					"session_id": "sess-abc123",
					"timestamp":  "2024-01-15T10:30:05Z",
				},
			},
			"total":    10,
			"limit":    50,
			"offset":   0,
			"has_more": false,
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	messages, err := client.GetMessages(context.Background(), "sess-abc123", nil)

	// Assert
	require.NoError(t, err)
	assert.Len(t, messages.Messages, 2)
	assert.Equal(t, int64(10), messages.Total)
	assert.Equal(t, int64(50), messages.Limit)
	assert.Equal(t, int64(0), messages.Offset)
	assert.False(t, messages.HasMore)

	// Check first message
	assert.Equal(t, "msg-001", messages.Messages[0].UUID)
	assert.Equal(t, "sess-abc123", messages.Messages[0].SessionID)
}

// TestGetMessages_WithPagination tests GetMessages with pagination options.
func TestGetMessages_WithPagination(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify pagination query params
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		assert.Equal(t, "50", r.URL.Query().Get("offset"))

		// Return mock response with has_more
		resp := map[string]interface{}{
			"messages": []map[string]interface{}{},
			"total":    100,
			"limit":    25,
			"offset":   50,
			"has_more": true,
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	messages, err := client.GetMessages(context.Background(), "sess-abc123", &stromboli.GetMessagesOptions{
		Limit:  25,
		Offset: 50,
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(100), messages.Total)
	assert.Equal(t, int64(25), messages.Limit)
	assert.Equal(t, int64(50), messages.Offset)
	assert.True(t, messages.HasMore)
}

// TestGetMessages_EmptySessionID tests GetMessages with an empty session ID.
func TestGetMessages_EmptySessionID(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	messages, err := client.GetMessages(context.Background(), "", nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, messages)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestGetMessage_Success tests the GetMessage method.
func TestGetMessage_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/sessions/sess-abc123/messages/msg-001", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"message": map[string]interface{}{
				"uuid":            "msg-001",
				"type":            "assistant",
				"session_id":      "sess-abc123",
				"cwd":             "/workspace",
				"git_branch":      "main",
				"permission_mode": "default",
				"timestamp":       "2024-01-15T10:30:00Z",
				"version":         "2.1.19",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client := stromboli.NewClient(server.URL)
	msg, err := client.GetMessage(context.Background(), "sess-abc123", "msg-001")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "msg-001", msg.UUID)
	assert.Equal(t, "sess-abc123", msg.SessionID)
	assert.Equal(t, "/workspace", msg.Cwd)
	assert.Equal(t, "main", msg.GitBranch)
	assert.Equal(t, "default", msg.PermissionMode)
	assert.Equal(t, "2.1.19", msg.Version)
}

// TestGetMessage_EmptySessionID tests GetMessage with an empty session ID.
func TestGetMessage_EmptySessionID(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	msg, err := client.GetMessage(context.Background(), "", "msg-001")

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestGetMessage_EmptyMessageID tests GetMessage with an empty message ID.
func TestGetMessage_EmptyMessageID(t *testing.T) {
	// Arrange
	client := stromboli.NewClient("http://localhost:8585")

	// Act
	msg, err := client.GetMessage(context.Background(), "sess-abc123", "")

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}
