//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// TestGetToken_E2E tests obtaining JWT tokens.
func TestGetToken_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	tokens, err := client.GetToken(ctx, "test-client-id")
	require.NoError(t, err, "GetToken should succeed")

	// Verify response structure
	assert.NotEmpty(t, tokens.AccessToken, "AccessToken should not be empty")
	assert.NotEmpty(t, tokens.TokenType, "TokenType should not be empty")

	t.Logf("Token obtained: type=%s, expires_in=%d", tokens.TokenType, tokens.ExpiresIn)
}

// TestRefreshToken_E2E tests refreshing access tokens.
func TestRefreshToken_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	// First get a token to get a refresh token
	initialTokens, err := client.GetToken(ctx, "test-client-id")
	require.NoError(t, err, "GetToken should succeed")

	// Skip if no refresh token (mock may not return one)
	if initialTokens.RefreshToken == "" {
		t.Skip("No refresh token returned, skipping refresh test")
	}

	// Refresh the token
	newTokens, err := client.RefreshToken(ctx, initialTokens.RefreshToken)
	require.NoError(t, err, "RefreshToken should succeed")

	assert.NotEmpty(t, newTokens.AccessToken, "New AccessToken should not be empty")
	t.Logf("Token refreshed: expires_in=%d", newTokens.ExpiresIn)
}

// TestValidateToken_E2E tests validating JWT tokens.
func TestValidateToken_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	// First get a token
	tokens, err := client.GetToken(ctx, "test-client-id")
	require.NoError(t, err, "GetToken should succeed")

	// Set the token and validate
	client.SetToken(tokens.AccessToken)
	validation, err := client.ValidateToken(ctx)
	require.NoError(t, err, "ValidateToken should succeed")

	t.Logf("Token validation: valid=%v, subject=%s, expires_at=%d",
		validation.Valid, validation.Subject, validation.ExpiresAt)
}

// TestLogout_E2E tests invalidating JWT tokens.
func TestLogout_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	// First get a token
	tokens, err := client.GetToken(ctx, "test-client-id")
	require.NoError(t, err, "GetToken should succeed")

	// Set the token and logout
	client.SetToken(tokens.AccessToken)
	result, err := client.Logout(ctx)
	require.NoError(t, err, "Logout should succeed")

	t.Logf("Logout result: success=%v, message=%s", result.Success, result.Message)
}

// TestAuthFlow_E2E tests the complete authentication flow.
func TestAuthFlow_E2E(t *testing.T) {
	skipIfMock(t, "Requires real server for full auth flow")

	client := newTestClient()
	ctx := newTestContext(t)

	// 1. Get token
	tokens, err := client.GetToken(ctx, "test-client-id")
	require.NoError(t, err, "GetToken should succeed")
	t.Logf("1. Got token: %s...", tokens.AccessToken[:20])

	// 2. Set and validate token
	client.SetToken(tokens.AccessToken)
	validation, err := client.ValidateToken(ctx)
	require.NoError(t, err, "ValidateToken should succeed")
	require.True(t, validation.Valid, "Token should be valid")
	t.Logf("2. Token valid: subject=%s", validation.Subject)

	// 3. Refresh token
	if tokens.RefreshToken != "" {
		newTokens, err := client.RefreshToken(ctx, tokens.RefreshToken)
		require.NoError(t, err, "RefreshToken should succeed")
		client.SetToken(newTokens.AccessToken)
		t.Log("3. Token refreshed")
	}

	// 4. Logout
	result, err := client.Logout(ctx)
	require.NoError(t, err, "Logout should succeed")
	require.True(t, result.Success, "Logout should succeed")
	t.Log("4. Logged out successfully")
}

// TestWithToken_Option tests creating a client with a pre-set token.
func TestWithToken_Option(t *testing.T) {
	// Create a server that checks for the auth header
	client, err := stromboli.NewClient(
		getBaseURL(),
		stromboli.WithToken("pre-set-token"),
	)
	require.NoError(t, err, "NewClient should succeed")
	ctx := newTestContext(t)

	// This should use the pre-set token
	// (The mock server will accept any token)
	validation, err := client.ValidateToken(ctx)
	require.NoError(t, err, "ValidateToken should succeed with pre-set token")

	t.Logf("Validation with pre-set token: valid=%v", validation.Valid)
}
