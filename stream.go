package stromboli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"strings"
	"sync/atomic"
)

// maxErrorBodySize limits the size of error response bodies read from the server.
// This prevents memory exhaustion from malicious or misconfigured servers that
// might return extremely large error responses. 4KB is sufficient for most
// error messages while providing a safety limit.
const maxErrorBodySize = 4096

// maxEventSize limits the maximum size of a single SSE event to prevent
// memory exhaustion from malformed or malicious servers that might send
// events without proper empty line delimiters.
const maxEventSize = 10 * 1024 * 1024 // 10MB

// StreamRequest represents a request for streaming Claude output.
//
// This is a simplified version of [RunRequest] for the streaming endpoint,
// which only supports a subset of options via query parameters.
type StreamRequest struct {
	// Prompt is the message to send to Claude. Required.
	Prompt string

	// Workdir is the working directory inside the container.
	Workdir string

	// SessionID enables conversation continuation.
	SessionID string
}

// StreamEvent represents a single event from the SSE stream.
//
// SSE events have an optional event type and data payload.
// Most events will have Type empty and Data containing the output.
type StreamEvent struct {
	// Type is the event type (from "event:" line).
	// Common types: "", "message", "error", "done"
	Type string

	// Data is the event payload (from "data:" line).
	Data string

	// ID is the event ID (from "id:" line), if provided.
	ID string
}

// Stream represents an active SSE stream from Claude.
//
// Use [Client.Stream] to create a stream, then iterate over events:
//
//	stream, err := client.Stream(ctx, &stromboli.StreamRequest{
//	    Prompt: "Count from 1 to 10",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close()
//
//	for stream.Next() {
//	    event := stream.Event()
//	    fmt.Print(event.Data)
//	}
//
//	if err := stream.Err(); err != nil {
//	    log.Fatal(err)
//	}
type Stream struct {
	resp    *http.Response
	reader  *bufio.Reader
	current *StreamEvent
	err     error
	closed  atomic.Bool
}

// Next advances to the next event in the stream.
//
// Returns true if an event is available, false if the stream is
// exhausted or an error occurred. Call [Stream.Err] to check for errors.
//
// Example:
//
//	for stream.Next() {
//	    event := stream.Event()
//	    fmt.Print(event.Data)
//	}
func (s *Stream) Next() bool {
	if s.closed.Load() || s.err != nil {
		return false
	}

	event, err := s.readEvent()
	if err != nil {
		if err != io.EOF {
			s.err = err
		}
		return false
	}

	s.current = event
	return true
}

// Event returns the current event.
//
// Call this after [Stream.Next] returns true.
func (s *Stream) Event() *StreamEvent {
	return s.current
}

// Err returns any error that occurred during streaming.
//
// Returns nil if the stream completed successfully or is still active.
func (s *Stream) Err() error {
	return s.err
}

// Close closes the stream and releases resources.
//
// Always call Close when done with the stream, preferably with defer.
// This is required even if [Stream.Next] returns false due to an error,
// as the underlying HTTP response body must be closed to release resources.
//
// Close is safe to call multiple times and is thread-safe.
//
// Example:
//
//	stream, err := client.Stream(ctx, req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close() // Always close, even on errors
func (s *Stream) Close() error {
	if s.closed.Swap(true) {
		return nil // Already closed
	}
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}

// EventsWithContext returns a channel that yields events from the stream.
//
// The channel is closed when the stream ends, an error occurs, or the
// context is cancelled. This is the preferred method to avoid goroutine
// leaks if you stop reading before the stream ends.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
//	defer cancel()
//
//	for event := range stream.EventsWithContext(ctx) {
//	    fmt.Print(event.Data)
//	}
//	if err := stream.Err(); err != nil {
//	    log.Fatal(err)
//	}
func (s *Stream) EventsWithContext(ctx context.Context) <-chan *StreamEvent {
	ch := make(chan *StreamEvent)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.err = fmt.Errorf("panic in stream reader: %v\n%s", r, debug.Stack())
			}
			close(ch)
		}()

		// Watch for context cancellation to close stream and unblock reader.
		// This prevents goroutine leaks when context is cancelled while
		// the reader is blocked on network I/O.
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = s.Close() // Unblocks the reader
			case <-done:
			}
		}()
		defer close(done)

		for s.Next() {
			// Copy the event to avoid race condition when consumer
			// reads while we iterate to the next event.
			event := *s.current
			select {
			case ch <- &event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// Events returns a channel that yields events from the stream.
//
// The channel is closed when the stream ends or an error occurs.
// Check [Stream.Err] after the channel closes to see if an error occurred.
//
// Deprecated: Use [Stream.EventsWithContext] to avoid goroutine leaks if you
// stop reading before the stream ends.
//
// Example:
//
//	for event := range stream.Events() {
//	    fmt.Print(event.Data)
//	}
//	if err := stream.Err(); err != nil {
//	    log.Fatal(err)
//	}
func (s *Stream) Events() <-chan *StreamEvent {
	return s.EventsWithContext(context.Background())
}

// readEvent reads the next SSE event from the stream.
func (s *Stream) readEvent() (*StreamEvent, error) {
	event := &StreamEvent{}
	hasData := false
	totalSize := 0

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && hasData {
				// Return the event we have so far
				return event, nil
			}
			return nil, err
		}

		// Track event size to prevent memory exhaustion from malformed streams
		totalSize += len(line)
		if totalSize > maxEventSize {
			return nil, fmt.Errorf("event exceeds maximum size of %d bytes", maxEventSize)
		}

		// Remove trailing newline
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Empty line marks end of event
		if line == "" {
			if hasData {
				return event, nil
			}
			continue
		}

		// Parse SSE field
		// Note: SSE spec says "retry:" sets reconnection time, but we intentionally
		// ignore it as this client doesn't implement auto-reconnection.
		switch {
		case strings.HasPrefix(line, "data:"):
			// Try with space first, then without
			data, found := strings.CutPrefix(line, "data: ")
			if !found {
				data, _ = strings.CutPrefix(line, "data:")
			}
			if hasData {
				event.Data += "\n" + data
			} else {
				event.Data = data
			}
			hasData = true
		case strings.HasPrefix(line, "event:"):
			var found bool
			event.Type, found = strings.CutPrefix(line, "event: ")
			if !found {
				event.Type, _ = strings.CutPrefix(line, "event:")
			}
		case strings.HasPrefix(line, "id:"):
			var found bool
			event.ID, found = strings.CutPrefix(line, "id: ")
			if !found {
				event.ID, _ = strings.CutPrefix(line, "id:")
			}
		}
		// Ignore "retry:" (reconnection time) and comments (lines starting with ":")
	}
}

// Stream executes Claude and streams output in real-time.
//
// This method connects to the SSE (Server-Sent Events) endpoint and
// returns a [Stream] that yields events as they arrive.
//
// # Timeout Behavior
//
// WARNING: The client timeout ([WithTimeout]) does NOT apply to streaming
// requests. If no context deadline is set and the server stops responding,
// this method may block indefinitely. Always use context.WithTimeout:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
//	defer cancel()
//	stream, err := client.Stream(ctx, req)
//
// The timeout behavior differs from regular requests because streams are
// designed for long-running connections where data arrives incrementally.
//
// # Basic Usage
//
//	stream, err := client.Stream(ctx, &stromboli.StreamRequest{
//	    Prompt: "Write a haiku about Go programming",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close()
//
//	for stream.Next() {
//	    fmt.Print(stream.Event().Data)
//	}
//
//	if err := stream.Err(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Channel Iteration
//
//	stream, _ := client.Stream(ctx, req)
//	defer stream.Close()
//
//	for event := range stream.Events() {
//	    fmt.Print(event.Data)
//	}
//
// # Continuing a Conversation
//
//	// First interaction
//	stream1, _ := client.Stream(ctx, &stromboli.StreamRequest{
//	    Prompt: "My name is Alice",
//	})
//	// ... consume stream1 ...
//	sessionID := "..." // Get from previous response
//
//	// Continue conversation
//	stream2, _ := client.Stream(ctx, &stromboli.StreamRequest{
//	    Prompt:    "What's my name?",
//	    SessionID: sessionID,
//	})
func (c *Client) Stream(ctx context.Context, req *StreamRequest) (*Stream, error) {
	if req == nil {
		return nil, newError("BAD_REQUEST", "request is required", 400, nil)
	}
	if req.Prompt == "" {
		return nil, newError("BAD_REQUEST", "prompt is required", 400, nil)
	}

	// Build URL with query parameters
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, newError("INVALID_URL", "invalid base URL", 0, err)
	}
	// Preserve any base path in the URL (e.g., /api/v1)
	u.Path = path.Join(u.Path, "run", "stream")

	query := u.Query()
	query.Set("prompt", req.Prompt)
	if req.Workdir != "" {
		query.Set("workdir", req.Workdir)
	}
	if req.SessionID != "" {
		query.Set("session_id", req.SessionID)
	}
	u.RawQuery = query.Encode()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, newError("REQUEST_FAILED", "failed to create request", 0, err)
	}

	// Set headers
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	httpReq.Header.Set("User-Agent", c.userAgent)

	// Add auth if token is set (thread-safe access)
	if token := c.getToken(); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Close response body if present to prevent resource leak
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, c.handleError(err, "failed to connect to stream")
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		// Limit body read to prevent memory exhaustion from large error responses
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return nil, newError(
			"STREAM_ERROR",
			fmt.Sprintf("stream request failed: %s", string(body)),
			resp.StatusCode,
			nil,
		)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		defer func() { _ = resp.Body.Close() }()
		return nil, newError(
			"INVALID_RESPONSE",
			fmt.Sprintf("unexpected content type: %s", contentType),
			resp.StatusCode,
			nil,
		)
	}

	return &Stream{
		resp:   resp,
		reader: bufio.NewReader(resp.Body),
	}, nil
}
