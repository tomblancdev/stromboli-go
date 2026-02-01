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
		json.NewEncoder(w).Encode(resp)
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
		json.NewEncoder(w).Encode(resp)
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
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
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
		json.NewEncoder(w).Encode(resp)
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
		json.NewEncoder(w).Encode(resp)
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
