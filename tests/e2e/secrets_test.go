//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestListSecrets_E2E tests listing available Podman secrets.
//
// Note: Prism returns mock "error" field with placeholder value,
// which our SDK interprets as an error. Skip for mock server.
func TestListSecrets_E2E(t *testing.T) {
	skipIfMock(t, "Prism returns placeholder error field in mock data")

	client := newTestClient()
	ctx := newTestContext(t)

	secrets, err := client.ListSecrets(ctx)
	require.NoError(t, err, "ListSecrets should succeed")

	// Log for debugging
	t.Logf("Found %d secrets", len(secrets))
	for i, name := range secrets {
		if i < 5 { // Only log first 5
			t.Logf("  - %s", name)
		}
	}
	if len(secrets) > 5 {
		t.Logf("  - ... and %d more", len(secrets)-5)
	}
}
