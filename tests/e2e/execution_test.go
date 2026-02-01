//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// TestRun_E2E tests synchronous execution against a real/mock server.
//
// Note: This test requires a real Stromboli server because Prism returns
// mock data that doesn't match the expected response format.
func TestRun_E2E(t *testing.T) {
	skipIfMock(t, "Run endpoint returns mismatched mock data")

	client := newTestClient()
	ctx := newTestContext(t)

	result, err := client.Run(ctx, &stromboli.RunRequest{
		Prompt: "Hello, Claude! Please respond with a short greeting.",
	})
	require.NoError(t, err, "Run should succeed")

	// Verify response structure
	assert.NotEmpty(t, result.ID, "ID should not be empty")
	assert.NotEmpty(t, result.Status, "Status should not be empty")

	// Log for debugging
	t.Logf("Run result: id=%s status=%s", result.ID, result.Status)
	if result.Output != "" {
		t.Logf("Output: %s", result.Output)
	}
	if result.Error != "" {
		t.Logf("Error: %s", result.Error)
	}
	if result.SessionID != "" {
		t.Logf("Session ID: %s", result.SessionID)
	}
}

// TestRun_WithOptions_E2E tests execution with various options.
func TestRun_WithOptions_E2E(t *testing.T) {
	skipIfMock(t, "Run endpoint returns mismatched mock data")

	client := newTestClient()
	ctx := newTestContext(t)

	result, err := client.Run(ctx, &stromboli.RunRequest{
		Prompt:  "What is 2+2?",
		Workdir: "/workspace",
		Claude: &stromboli.ClaudeOptions{
			Model:        stromboli.ModelHaiku,
			MaxBudgetUSD: 1.0,
		},
		Podman: &stromboli.PodmanOptions{
			Memory:  "512m",
			Timeout: "1m",
		},
	})
	require.NoError(t, err, "Run with options should succeed")

	// Verify response structure
	assert.NotEmpty(t, result.ID, "ID should not be empty")
	assert.NotEmpty(t, result.Status, "Status should not be empty")

	t.Logf("Run with options: id=%s status=%s", result.ID, result.Status)
}

// TestRunAsync_E2E tests asynchronous execution.
func TestRunAsync_E2E(t *testing.T) {
	skipIfMock(t, "RunAsync endpoint returns mismatched mock data")

	client := newTestClient()
	ctx := newTestContext(t)

	result, err := client.RunAsync(ctx, &stromboli.RunRequest{
		Prompt: "Analyze this codebase and provide a summary.",
	})
	require.NoError(t, err, "RunAsync should succeed")

	// Verify response structure
	assert.NotEmpty(t, result.JobID, "JobID should not be empty")

	t.Logf("Async job started: job_id=%s", result.JobID)
}
