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
