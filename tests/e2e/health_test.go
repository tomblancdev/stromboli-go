//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealth_E2E tests the Health endpoint against a real/mock server.
func TestHealth_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	health, err := client.Health(ctx)
	require.NoError(t, err, "Health check should succeed")

	// Verify response structure
	assert.NotEmpty(t, health.Name, "Name should not be empty")
	assert.NotEmpty(t, health.Status, "Status should not be empty")
	assert.NotEmpty(t, health.Version, "Version should not be empty")

	// Log for debugging
	t.Logf("Health: name=%s status=%s version=%s", health.Name, health.Status, health.Version)
	t.Logf("Components: %d", len(health.Components))
	for _, c := range health.Components {
		t.Logf("  - %s: %s", c.Name, c.Status)
	}
}

// TestClaudeStatus_E2E tests the Claude status endpoint.
func TestClaudeStatus_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	status, err := client.ClaudeStatus(ctx)
	require.NoError(t, err, "Claude status check should succeed")

	// Verify response structure
	assert.NotEmpty(t, status.Message, "Message should not be empty")

	// Log for debugging
	t.Logf("Claude Status: configured=%v message=%s", status.Configured, status.Message)
}
