package unit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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

	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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

	client, err := stromboli.NewClient("http://localhost:8585",
		stromboli.WithTimeout(60),
		stromboli.WithRetries(3),
		stromboli.WithHTTPClient(customHTTPClient),
		stromboli.WithUserAgent("test-agent/1.0"),
	)
	require.NoError(t, err)

	assert.NotNil(t, client)
}

// TestNewClient_InvalidURL tests NewClient with an invalid URL.
func TestNewClient_InvalidURL(t *testing.T) {
	_, err := stromboli.NewClient("://invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")
}

// TestNewClient_MissingHost tests NewClient with a URL missing the host.
func TestNewClient_MissingHost(t *testing.T) {
	_, err := stromboli.NewClient("/just/a/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must include host")
}

// TestNewClient_InvalidScheme tests NewClient with unsupported URL schemes.
func TestNewClient_InvalidScheme(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com"},
		{"ws scheme", "ws://example.com"},
		{"custom scheme", "custom://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := stromboli.NewClient(tt.url)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported URL scheme")
		})
	}
}

// TestNewClient_ValidURL tests NewClient with various valid URLs.
func TestNewClient_ValidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost:8585"},
		{"https", "https://stromboli.example.com"},
		{"ip address", "http://192.168.1.1:8585"},
		{"with path", "http://localhost:8585/api/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := stromboli.NewClient(tt.url)
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestClient_ConcurrentTokenAccess tests that SetToken is thread-safe.
// Run with -race flag to detect data races.
func TestClient_ConcurrentTokenAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 401 for any request - we're just testing that it doesn't race
		w.WriteHeader(http.StatusUnauthorized)
		mustEncode(w, map[string]interface{}{
			"error": "unauthorized",
		})
	}))
	defer server.Close()

	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			client.SetToken(fmt.Sprintf("token-%d", i))
		}(i)
		go func() {
			defer wg.Done()
			// ValidateToken reads the token and makes a request
			_, _ = client.ValidateToken(context.Background())
		}()
	}
	wg.Wait()
	// Test passes if no race detected (run with -race flag)
}

// TestError_Is tests the Error.Is implementation.
func TestError_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      *stromboli.Error
		target   error
		expected bool
	}{
		{
			name:     "same code matches",
			err:      &stromboli.Error{Code: "NOT_FOUND", Message: "specific message"},
			target:   stromboli.ErrNotFound,
			expected: true,
		},
		{
			name:     "different code does not match",
			err:      &stromboli.Error{Code: "NOT_FOUND"},
			target:   stromboli.ErrTimeout,
			expected: false,
		},
		{
			name:     "non-Error target does not match",
			err:      &stromboli.Error{Code: "NOT_FOUND"},
			target:   fmt.Errorf("some error"),
			expected: false,
		},
		{
			name:     "nil target does not match",
			err:      &stromboli.Error{Code: "NOT_FOUND"},
			target:   nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.err, tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestError_Is_WithWrappedError tests errors.Is with wrapped errors.
func TestError_Is_WithWrappedError(t *testing.T) {
	// Create an error with a cause
	wrappedErr := &stromboli.Error{
		Code:    "NOT_FOUND",
		Message: "resource not found",
		Cause:   fmt.Errorf("underlying error"),
	}

	// errors.Is should match the error code
	assert.True(t, errors.Is(wrappedErr, stromboli.ErrNotFound))
	assert.False(t, errors.Is(wrappedErr, stromboli.ErrTimeout))
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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

// TestRun_WithJSONSchema tests Run with JSON schema for structured output.
func TestRun_WithJSONSchema(t *testing.T) {
	schema := `{"type":"object","required":["summary"],"properties":{"summary":{"type":"string"}}}`

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/run", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)

		// Verify Claude options include JSONSchema
		claude, ok := req["claude"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "json", claude["output_format"])

		// Use JSONEq for robust JSON comparison (handles formatting differences)
		actualSchema, ok := claude["json_schema"].(string)
		require.True(t, ok)
		assert.JSONEq(t, schema, actualSchema)

		// Return mock response with structured output
		resp := map[string]interface{}{
			"id":         "run-json123",
			"status":     "completed",
			"output":     `{"summary":"Test completed successfully"}`,
			"session_id": "sess-json789",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Summarize this text",
		Claude: &stromboli.ClaudeOptions{
			OutputFormat: "json",
			JSONSchema:   schema,
		},
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "run-json123", result.ID)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.Output, "summary")
	assert.True(t, result.IsSuccess())
}

// TestRun_WithJSONSchema_ComplexSchema tests Run with a complex multi-line JSON schema.
func TestRun_WithJSONSchema_ComplexSchema(t *testing.T) {
	// Complex schema with nested types and arrays
	schema := `{
		"type": "object",
		"required": ["items", "metadata"],
		"properties": {
			"items": {
				"type": "array",
				"items": {"type": "string"}
			},
			"metadata": {
				"type": "object",
				"properties": {
					"count": {"type": "integer"},
					"valid": {"type": "boolean"}
				}
			}
		}
	}`

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)

		// Verify Claude options include JSONSchema
		claude, ok := req["claude"].(map[string]interface{})
		require.True(t, ok)

		actualSchema, ok := claude["json_schema"].(string)
		require.True(t, ok)
		assert.JSONEq(t, schema, actualSchema)

		// Return mock response
		resp := map[string]interface{}{
			"id":     "run-complex123",
			"status": "completed",
			"output": `{"items":["a","b"],"metadata":{"count":2,"valid":true}}`,
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "List items",
		Claude: &stromboli.ClaudeOptions{
			OutputFormat: "json",
			JSONSchema:   schema,
		},
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())
}

// TestRun_WithJSONSchema_NoOutputFormat tests that JSONSchema works without explicit OutputFormat.
func TestRun_WithJSONSchema_NoOutputFormat(t *testing.T) {
	schema := `{"type":"object","properties":{"result":{"type":"string"}}}`

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)

		// Verify Claude options include JSONSchema but no output_format
		claude, ok := req["claude"].(map[string]interface{})
		require.True(t, ok)

		actualSchema, ok := claude["json_schema"].(string)
		require.True(t, ok)
		assert.JSONEq(t, schema, actualSchema)

		// output_format should not be set (or empty)
		_, hasOutputFormat := claude["output_format"]
		assert.False(t, hasOutputFormat, "output_format should not be set when not provided")

		// Return mock response
		resp := map[string]interface{}{
			"id":     "run-noformat123",
			"status": "completed",
			"output": `{"result":"success"}`,
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Do something",
		Claude: &stromboli.ClaudeOptions{
			JSONSchema: schema,
			// Note: OutputFormat intentionally not set
		},
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())
}

// TestRun_WithEmptyJSONSchema tests that empty JSONSchema is omitted from the request.
func TestRun_WithEmptyJSONSchema(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req map[string]interface{}
		mustDecode(r, &req)

		// Verify Claude options exist but json_schema is not present
		claude, ok := req["claude"].(map[string]interface{})
		require.True(t, ok)

		// json_schema should not be present when empty
		_, hasJSONSchema := claude["json_schema"]
		assert.False(t, hasJSONSchema, "json_schema should be omitted when empty")

		// Return mock response
		resp := map[string]interface{}{
			"id":     "run-empty123",
			"status": "completed",
			"output": "Plain text response",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Do something",
		Claude: &stromboli.ClaudeOptions{
			Model:      stromboli.ModelHaiku,
			JSONSchema: "", // Explicitly empty
		},
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())
}

// TestRun_EmptyPrompt tests that Run returns an error when prompt is empty.
func TestRun_EmptyPrompt(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	err = client.CancelJob(context.Background(), "job-cancel123")

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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	err = client.CancelJob(context.Background(), "invalid-id")

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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	err = client.CancelJob(context.Background(), "")

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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	err = client.DestroySession(context.Background(), "sess-abc123")

	// Assert
	require.NoError(t, err)
}

// TestDestroySession_EmptyID tests DestroySession with an empty session ID.
func TestDestroySession_EmptyID(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	err = client.DestroySession(context.Background(), "")

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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

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
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

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
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	msg, err := client.GetMessage(context.Background(), "sess-abc123", "")

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// ============================================================================
// Auth Tests
// ============================================================================

// TestGetToken_Success tests the GetToken method.
func TestGetToken_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/auth/token", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			"refresh_token": "refresh_abc123",
			"expires_in":    3600,
			"token_type":    "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	tokens, err := client.GetToken(context.Background(), "my-client-id")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", tokens.AccessToken)
	assert.Equal(t, "refresh_abc123", tokens.RefreshToken)
	assert.Equal(t, int64(3600), tokens.ExpiresIn)
	assert.Equal(t, "Bearer", tokens.TokenType)
}

// TestGetToken_EmptyClientID tests GetToken with an empty client ID.
func TestGetToken_EmptyClientID(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	tokens, err := client.GetToken(context.Background(), "")

	// Assert
	require.Error(t, err)
	assert.Nil(t, tokens)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestRefreshToken_Success tests the RefreshToken method.
func TestRefreshToken_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/auth/refresh", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"access_token":  "new_access_token_xyz",
			"refresh_token": "new_refresh_token_xyz",
			"expires_in":    3600,
			"token_type":    "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	tokens, err := client.RefreshToken(context.Background(), "old_refresh_token")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "new_access_token_xyz", tokens.AccessToken)
	assert.Equal(t, "new_refresh_token_xyz", tokens.RefreshToken)
	assert.Equal(t, int64(3600), tokens.ExpiresIn)
}

// TestRefreshToken_EmptyToken tests RefreshToken with an empty token.
func TestRefreshToken_EmptyToken(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	tokens, err := client.RefreshToken(context.Background(), "")

	// Assert
	require.Error(t, err)
	assert.Nil(t, tokens)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestValidateToken_Success tests the ValidateToken method.
func TestValidateToken_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/auth/validate", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Verify auth header
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer test-token-123", authHeader)

		// Return mock response
		resp := map[string]interface{}{
			"valid":      true,
			"subject":    "my-client-id",
			"expires_at": 1704067200, // 2024-01-01 00:00:00 UTC
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL, stromboli.WithToken("test-token-123"))
	require.NoError(t, err)
	validation, err := client.ValidateToken(context.Background())

	// Assert
	require.NoError(t, err)
	assert.True(t, validation.Valid)
	assert.Equal(t, "my-client-id", validation.Subject)
	assert.Equal(t, int64(1704067200), validation.ExpiresAt)
}

// TestValidateToken_NoToken tests ValidateToken without a token set.
func TestValidateToken_NoToken(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	validation, err := client.ValidateToken(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, validation)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "UNAUTHORIZED", apiErr.Code)
}

// TestLogout_Success tests the Logout method.
func TestLogout_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/auth/logout", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify auth header
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer test-token-123", authHeader)

		// Return mock response
		resp := map[string]interface{}{
			"success": true,
			"message": "Token invalidated successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL, stromboli.WithToken("test-token-123"))
	require.NoError(t, err)
	result, err := client.Logout(context.Background())

	// Assert
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Token invalidated successfully", result.Message)
}

// TestLogout_NoToken tests Logout without a token set.
func TestLogout_NoToken(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	result, err := client.Logout(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "UNAUTHORIZED", apiErr.Code)
}

// TestSetToken tests the SetToken method.
func TestSetToken(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header after SetToken
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer new-token-456", authHeader)

		resp := map[string]interface{}{
			"valid":      true,
			"subject":    "test",
			"expires_at": 1704067200,
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	client.SetToken("new-token-456")
	validation, err := client.ValidateToken(context.Background())

	// Assert
	require.NoError(t, err)
	assert.True(t, validation.Valid)
}

// ============================================================================
// Secrets Tests
// ============================================================================

// TestListSecrets_Success tests the ListSecrets method.
func TestListSecrets_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/secrets", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return mock response
		resp := map[string]interface{}{
			"secrets": []string{"github-token", "gitlab-token", "npm-token"},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	secrets, err := client.ListSecrets(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, secrets, 3)
	assert.Contains(t, secrets, "github-token")
	assert.Contains(t, secrets, "gitlab-token")
	assert.Contains(t, secrets, "npm-token")
}

// TestListSecrets_Empty tests ListSecrets when no secrets exist.
func TestListSecrets_Empty(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"secrets": []string{},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	secrets, err := client.ListSecrets(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Empty(t, secrets)
}

// TestListSecrets_Error tests ListSecrets when the server returns an error.
func TestListSecrets_Error(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"secrets": []string{},
			"error":   "podman not available",
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncode(w, resp)
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	secrets, err := client.ListSecrets(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, secrets)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "SECRETS_ERROR", apiErr.Code)
	assert.Contains(t, apiErr.Message, "podman not available")
}

// ============================================================================
// Streaming Tests
// ============================================================================

// TestStream_Success tests the Stream method with SSE events.
func TestStream_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/run/stream", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Hello", r.URL.Query().Get("prompt"))
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		// Send SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "ResponseWriter should be a Flusher")

		// Send events
		_, _ = fmt.Fprintf(w, "data: Hello\n\n")
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "data: World\n\n")
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "event: done\ndata: \n\n")
		flusher.Flush()
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "Hello",
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, stream)
	defer func() { _ = stream.Close() }()

	// Collect events
	var events []*stromboli.StreamEvent
	for stream.Next() {
		events = append(events, stream.Event())
	}
	require.NoError(t, stream.Err())

	assert.Len(t, events, 3)
	assert.Equal(t, "Hello", events[0].Data)
	assert.Equal(t, "World", events[1].Data)
	assert.Equal(t, "done", events[2].Type)
}

// TestStream_WithOptions tests streaming with workdir and session_id.
func TestStream_WithOptions(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters
		assert.Equal(t, "Test prompt", r.URL.Query().Get("prompt"))
		assert.Equal(t, "/workspace", r.URL.Query().Get("workdir"))
		assert.Equal(t, "sess-123", r.URL.Query().Get("session_id"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "data: OK\n\n")
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt:    "Test prompt",
		Workdir:   "/workspace",
		SessionID: "sess-123",
	})

	// Assert
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	require.True(t, stream.Next())
	assert.Equal(t, "OK", stream.Event().Data)
}

// TestStream_EmptyPrompt tests Stream with an empty prompt.
func TestStream_EmptyPrompt(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "",
	})

	// Assert
	require.Error(t, err)
	assert.Nil(t, stream)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestStream_NilRequest tests Stream with a nil request.
func TestStream_NilRequest(t *testing.T) {
	// Arrange
	client, err := stromboli.NewClient("http://localhost:8585")
	require.NoError(t, err)

	// Act
	stream, err := client.Stream(context.Background(), nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, stream)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestStream_ServerError tests Stream when the server returns an error.
func TestStream_ServerError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid prompt"))
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "Test",
	})

	// Assert
	require.Error(t, err)
	assert.Nil(t, stream)

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "STREAM_ERROR", apiErr.Code)
	assert.Equal(t, 400, apiErr.Status)
}

// TestStream_EventsChannel tests the Events() channel method.
func TestStream_EventsChannel(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)
		for i := 1; i <= 3; i++ {
			_, _ = fmt.Fprintf(w, "data: Line %d\n\n", i)
			flusher.Flush()
		}
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "Test",
	})
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// Collect events via channel
	events := make([]*stromboli.StreamEvent, 0, 3)
	for event := range stream.Events() {
		events = append(events, event)
	}

	// Assert
	require.NoError(t, stream.Err())
	assert.Len(t, events, 3)
	assert.Equal(t, "Line 1", events[0].Data)
	assert.Equal(t, "Line 2", events[1].Data)
	assert.Equal(t, "Line 3", events[2].Data)
}

// TestStream_MultilineData tests SSE events with multiline data.
func TestStream_MultilineData(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send multiline data (each line prefixed with "data:")
		_, _ = fmt.Fprintf(w, "data: Line 1\n")
		_, _ = fmt.Fprintf(w, "data: Line 2\n")
		_, _ = fmt.Fprintf(w, "data: Line 3\n")
		_, _ = fmt.Fprintf(w, "\n") // End of event
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "Test",
	})
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// Assert
	require.True(t, stream.Next())
	assert.Equal(t, "Line 1\nLine 2\nLine 3", stream.Event().Data)
}

// TestStream_WithEventType tests SSE events with event type.
func TestStream_WithEventType(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprintf(w, "event: message\n")
		_, _ = fmt.Fprintf(w, "id: 123\n")
		_, _ = fmt.Fprintf(w, "data: Hello\n")
		_, _ = fmt.Fprintf(w, "\n")
	}))
	defer server.Close()

	// Act
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "Test",
	})
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// Assert
	require.True(t, stream.Next())
	event := stream.Event()
	assert.Equal(t, "message", event.Type)
	assert.Equal(t, "123", event.ID)
	assert.Equal(t, "Hello", event.Data)
}

// TestStream_ContextCancellation tests that streams respect context cancellation.
func TestStream_ContextCancellation(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)
		// Send one event then wait
		_, _ = fmt.Fprintf(w, "data: First\n\n")
		flusher.Flush()

		// Wait for context cancellation (this would block forever otherwise)
		<-r.Context().Done()
	}))
	defer server.Close()

	// Act
	ctx, cancel := context.WithCancel(context.Background())
	client, err := stromboli.NewClient(server.URL)
	require.NoError(t, err)
	stream, err := client.Stream(ctx, &stromboli.StreamRequest{
		Prompt: "Test",
	})
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// Get first event
	require.True(t, stream.Next())
	assert.Equal(t, "First", stream.Event().Data)

	// Cancel context
	cancel()

	// Next should return false (stream closed due to cancellation)
	assert.False(t, stream.Next())
}
