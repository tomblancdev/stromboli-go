//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
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
	for i, s := range secrets {
		if i < 5 { // Only log first 5
			t.Logf("  - %s (id: %s)", s.Name, s.ID)
		}
	}
	if len(secrets) > 5 {
		t.Logf("  - ... and %d more", len(secrets)-5)
	}
}

// TestCreateSecret_E2E tests creating a new secret.
func TestCreateSecret_E2E(t *testing.T) {
	skipIfMock(t, "CreateSecret requires real Podman")

	client := newTestClient()
	ctx := newTestContext(t)

	secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())

	err := client.CreateSecret(ctx, &stromboli.CreateSecretRequest{
		Name:  secretName,
		Value: "test-value",
	})
	require.NoError(t, err, "CreateSecret should succeed")

	// Cleanup
	t.Cleanup(func() {
		_ = client.DeleteSecret(ctx, secretName)
	})

	t.Logf("Created secret: %s", secretName)
}

// TestGetSecret_E2E tests retrieving a secret.
func TestGetSecret_E2E(t *testing.T) {
	skipIfMock(t, "GetSecret requires real Podman")

	client := newTestClient()
	ctx := newTestContext(t)

	// First create a secret
	secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())
	err := client.CreateSecret(ctx, &stromboli.CreateSecretRequest{
		Name:  secretName,
		Value: "test-value",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = client.DeleteSecret(ctx, secretName)
	})

	// Get it
	secret, err := client.GetSecret(ctx, secretName)
	require.NoError(t, err, "GetSecret should succeed")
	require.NotNil(t, secret)
	assert.Equal(t, secretName, secret.Name)

	t.Logf("Got secret: %s (ID: %s)", secret.Name, secret.ID)
}

// TestDeleteSecret_E2E tests deleting a secret.
func TestDeleteSecret_E2E(t *testing.T) {
	skipIfMock(t, "DeleteSecret requires real Podman")

	client := newTestClient()
	ctx := newTestContext(t)

	// First create a secret
	secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())
	err := client.CreateSecret(ctx, &stromboli.CreateSecretRequest{
		Name:  secretName,
		Value: "test-value",
	})
	require.NoError(t, err)

	// Delete it
	err = client.DeleteSecret(ctx, secretName)
	require.NoError(t, err, "DeleteSecret should succeed")

	// Verify it's gone
	_, err = client.GetSecret(ctx, secretName)
	require.Error(t, err, "GetSecret should fail after delete")
	assert.True(t, errors.Is(err, stromboli.ErrNotFound), "Expected ErrNotFound")

	t.Logf("Deleted secret: %s", secretName)
}
