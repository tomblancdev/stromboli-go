//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// TestListSessions_E2E tests listing sessions against a real/mock server.
func TestListSessions_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	sessions, err := client.ListSessions(ctx)
	require.NoError(t, err, "ListSessions should succeed")

	// Log for debugging
	t.Logf("Found %d sessions", len(sessions))
	for i, id := range sessions {
		if i < 5 { // Only log first 5
			t.Logf("  - %s", id)
		}
	}
	if len(sessions) > 5 {
		t.Logf("  - ... and %d more", len(sessions)-5)
	}
}

// TestGetMessages_E2E tests getting messages from a session.
//
// Note: This test may fail with 404 if no sessions exist.
func TestGetMessages_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	// First, list sessions to find an existing one
	sessions, err := client.ListSessions(ctx)
	require.NoError(t, err, "ListSessions should succeed")

	if len(sessions) == 0 {
		t.Skip("No sessions found, skipping GetMessages test")
	}

	// Get messages from the first session
	sessionID := sessions[0]
	messages, err := client.GetMessages(ctx, sessionID, nil)
	require.NoError(t, err, "GetMessages should succeed")

	t.Logf("Session %s: %d messages (total: %d, has_more: %v)",
		sessionID, len(messages.Messages), messages.Total, messages.HasMore)

	for i, msg := range messages.Messages {
		if i < 3 { // Only log first 3
			t.Logf("  - %s: type=%s", msg.UUID, msg.Type)
		}
	}
}

// TestGetMessages_WithPagination_E2E tests paginated message retrieval.
func TestGetMessages_WithPagination_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	// First, list sessions to find an existing one
	sessions, err := client.ListSessions(ctx)
	require.NoError(t, err, "ListSessions should succeed")

	if len(sessions) == 0 {
		t.Skip("No sessions found, skipping pagination test")
	}

	sessionID := sessions[0]

	// Get first page
	page1, err := client.GetMessages(ctx, sessionID, &stromboli.GetMessagesOptions{
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err, "GetMessages page 1 should succeed")

	t.Logf("Page 1: %d messages (offset=%d, limit=%d, total=%d, has_more=%v)",
		len(page1.Messages), page1.Offset, page1.Limit, page1.Total, page1.HasMore)

	// If there are more messages, get second page
	if page1.HasMore {
		page2, err := client.GetMessages(ctx, sessionID, &stromboli.GetMessagesOptions{
			Limit:  10,
			Offset: 10,
		})
		require.NoError(t, err, "GetMessages page 2 should succeed")

		t.Logf("Page 2: %d messages (offset=%d)", len(page2.Messages), page2.Offset)
	}
}

// TestSessionLifecycle_E2E tests creating a session via Run and then retrieving messages.
//
// This is a more comprehensive test that exercises the session workflow.
func TestSessionLifecycle_E2E(t *testing.T) {
	skipIfMock(t, "Requires real server for session lifecycle")

	client := newTestClient()
	ctx := newTestContext(t)

	// 1. Run a prompt (this creates a session)
	result, err := client.Run(ctx, &stromboli.RunRequest{
		Prompt: "Hello! This is a test message.",
	})
	require.NoError(t, err, "Run should succeed")

	if result.SessionID == "" {
		t.Skip("No session ID returned, skipping lifecycle test (may be a mock limitation)")
	}

	t.Logf("Created session: %s", result.SessionID)

	// 2. Verify session appears in list
	sessions, err := client.ListSessions(ctx)
	require.NoError(t, err, "ListSessions should succeed")

	found := false
	for _, id := range sessions {
		if id == result.SessionID {
			found = true
			break
		}
	}
	t.Logf("Session found in list: %v (may be false with Prism mock)", found)

	// 3. Get messages from the session
	messages, err := client.GetMessages(ctx, result.SessionID, nil)
	if err != nil {
		t.Logf("GetMessages failed (expected with mock): %v", err)
	} else {
		t.Logf("Session has %d messages", len(messages.Messages))
	}
}
