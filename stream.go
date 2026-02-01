package stromboli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

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
	closed  bool
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
	if s.closed || s.err != nil {
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
// Always call Close when done with the stream, preferably with defer:
//
//	stream, err := client.Stream(ctx, req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close()
func (s *Stream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}

// Events returns a channel that yields events from the stream.
//
// The channel is closed when the stream ends or an error occurs.
// Check [Stream.Err] after the channel closes to see if an error occurred.
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
	ch := make(chan *StreamEvent)
	go func() {
		defer close(ch)
		for s.Next() {
			ch <- s.current
		}
	}()
	return ch
}

// readEvent reads the next SSE event from the stream.
func (s *Stream) readEvent() (*StreamEvent, error) {
	event := &StreamEvent{}
	hasData := false

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && hasData {
				// Return the event we have so far
				return event, nil
			}
			return nil, err
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
		switch {
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimPrefix(data, " ") // Optional space after colon
			if hasData {
				event.Data += "\n" + data
			} else {
				event.Data = data
			}
			hasData = true
		case strings.HasPrefix(line, "event:"):
			event.Type = strings.TrimPrefix(line, "event:")
			event.Type = strings.TrimPrefix(event.Type, " ")
		case strings.HasPrefix(line, "id:"):
			event.ID = strings.TrimPrefix(line, "id:")
			event.ID = strings.TrimPrefix(event.ID, " ")
		}
		// Ignore "retry:" and comments (lines starting with ":")
	}
}

// Stream executes Claude and streams output in real-time.
//
// This method connects to the SSE (Server-Sent Events) endpoint and
// returns a [Stream] that yields events as they arrive.
//
// Basic usage:
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
// Using channel iteration:
//
//	stream, _ := client.Stream(ctx, req)
//	defer stream.Close()
//
//	for event := range stream.Events() {
//	    fmt.Print(event.Data)
//	}
//
// Continuing a conversation:
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
	u.Path = "/run/stream"

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

	// Add auth if token is set
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, c.handleError(err, "failed to connect to stream")
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
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
		defer resp.Body.Close()
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
