//go:build e2e

// Package e2e provides end-to-end tests for the Stromboli Go SDK.
//
// These tests run against a real Stromboli server or a Prism mock server.
// By default, they connect to http://localhost:4010 (Prism default).
//
// To run against a real Stromboli instance:
//
//	STROMBOLI_URL=http://localhost:8585 make test-e2e
//
// To run against Prism mock server:
//
//	# Terminal 1: Start mock server
//	make mock-server
//
//	# Terminal 2: Run E2E tests
//	make test-e2e
//
// Note: Some tests may fail with Prism because it generates mock data that
// doesn't always match complex nested struct types. Set STROMBOLI_REAL=1 to
// run tests that require a real Stromboli server.
package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tomblancdev/stromboli-go"
)

// getBaseURL returns the Stromboli API base URL.
// It reads from STROMBOLI_URL environment variable, defaulting to Prism's port.
func getBaseURL() string {
	if url := os.Getenv("STROMBOLI_URL"); url != "" {
		return url
	}
	return "http://localhost:4010" // Prism default
}

// isRealServer returns true if running against a real Stromboli server.
// Set STROMBOLI_REAL=1 to indicate a real server.
func isRealServer() bool {
	return os.Getenv("STROMBOLI_REAL") == "1"
}

// skipIfMock skips the test if running against a mock server.
// Use this for tests that require real Stromboli behavior.
func skipIfMock(t *testing.T, reason string) {
	if !isRealServer() {
		t.Skipf("Skipping: %s (set STROMBOLI_REAL=1 for real server)", reason)
	}
}

// newTestClient creates a client configured for E2E testing.
func newTestClient() *stromboli.Client {
	return stromboli.NewClient(getBaseURL(),
		stromboli.WithTimeout(30*time.Second),
	)
}

// newTestContext creates a context with a reasonable timeout for E2E tests.
func newTestContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}
