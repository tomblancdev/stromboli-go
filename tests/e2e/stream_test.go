//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// TestStream_E2E tests SSE streaming against a real Stromboli server.
//
// Note: Prism mock server doesn't support SSE streaming, so this test
// requires a real Stromboli instance.
func TestStream_E2E(t *testing.T) {
	skipIfMock(t, "Prism doesn't support SSE streaming")

	client := newTestClient()
	ctx := newTestContext(t)

	stream, err := client.Stream(ctx, &stromboli.StreamRequest{
		Prompt: "Count from 1 to 5, each number on a new line.",
	})
	require.NoError(t, err, "Stream should connect successfully")
	defer stream.Close()

	// Collect all events
	var events []*stromboli.StreamEvent
	for stream.Next() {
		event := stream.Event()
		events = append(events, event)
		t.Logf("Event: type=%q data=%q", event.Type, event.Data)
	}

	require.NoError(t, stream.Err(), "Stream should complete without error")
	assert.NotEmpty(t, events, "Should receive at least one event")
}

// TestStream_WithSession_E2E tests streaming with session continuation.
func TestStream_WithSession_E2E(t *testing.T) {
	skipIfMock(t, "Prism doesn't support SSE streaming")

	client := newTestClient()
	ctx := newTestContext(t)

	// First interaction
	stream1, err := client.Stream(ctx, &stromboli.StreamRequest{
		Prompt: "My name is Alice. Remember this.",
	})
	require.NoError(t, err, "First stream should connect")

	// Consume stream1
	var output1 string
	for stream1.Next() {
		output1 += stream1.Event().Data
	}
	stream1.Close()
	require.NoError(t, stream1.Err(), "First stream should complete")
	t.Logf("First response: %s", output1)

	// Note: Session ID would come from stream metadata in real implementation
	// For now, this test validates the basic streaming flow
}

// TestStream_ChannelIteration_E2E tests the Events() channel method.
func TestStream_ChannelIteration_E2E(t *testing.T) {
	skipIfMock(t, "Prism doesn't support SSE streaming")

	client := newTestClient()
	ctx := newTestContext(t)

	stream, err := client.Stream(ctx, &stromboli.StreamRequest{
		Prompt: "Say hello.",
	})
	require.NoError(t, err, "Stream should connect")
	defer stream.Close()

	// Use channel iteration
	var collected string
	for event := range stream.Events() {
		collected += event.Data
	}

	require.NoError(t, stream.Err(), "Stream should complete without error")
	t.Logf("Collected output: %s", collected)
}
