# Stromboli Go SDK 🌋

[![Go Reference](https://pkg.go.dev/badge/github.com/tomblancdev/stromboli-go.svg)](https://pkg.go.dev/github.com/tomblancdev/stromboli-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/tomblancdev/stromboli-go)](https://goreportcard.com/report/github.com/tomblancdev/stromboli-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Official Go SDK for [Stromboli](https://github.com/tomblancdev/stromboli) — Container orchestration for Claude Code agents.

Stromboli provides a secure, isolated environment for running Claude Code in Podman containers. This SDK offers a clean, idiomatic Go interface to interact with the Stromboli API.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [API Reference](#api-reference)
  - [Client Configuration](#client-configuration)
  - [Execution](#execution)
  - [Streaming](#streaming)
  - [Jobs](#jobs)
  - [Sessions](#sessions)
  - [Authentication](#authentication)
  - [System](#system)
- [Error Handling](#error-handling)
- [Examples](#examples)
- [Development](#development)
- [License](#license)

## Features

- **Full API Coverage** — All Stromboli endpoints supported
- **Type Safety** — Strongly typed requests and responses
- **Streaming** — Real-time SSE streaming for Claude output
- **Context Support** — Cancellation and timeouts via context
- **Retries** — Automatic retries with exponential backoff
- **Idiomatic Go** — Follows Go best practices and conventions

## Installation

```bash
go get github.com/tomblancdev/stromboli-go
```

**Requirements:**
- Go 1.22 or later
- A running [Stromboli](https://github.com/tomblancdev/stromboli) instance

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/tomblancdev/stromboli-go"
)

func main() {
    // Create a client
    client := stromboli.NewClient("http://localhost:8585")

    // Execute Claude synchronously
    result, err := client.Run(context.Background(), &stromboli.RunRequest{
        Prompt: "Hello, Claude! Write a haiku about Go programming.",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Output)
}
```

## Core Concepts

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Your Application                          │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Stromboli Go SDK                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   Client    │  │   Types     │  │  Streaming  │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼ HTTP/SSE
┌─────────────────────────────────────────────────────────────────┐
│                      Stromboli API Server                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │    Jobs     │  │  Sessions   │  │   Secrets   │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Podman Containers                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                   Claude Code Agent                      │    │
│  │  • Isolated execution environment                        │    │
│  │  • Resource limits (CPU, memory)                         │    │
│  │  • Volume mounts for workspace access                    │    │
│  │  • Secret injection                                      │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### Execution Modes

| Mode | Method | Use Case |
|------|--------|----------|
| **Synchronous** | `Run()` | Short tasks, immediate response needed |
| **Asynchronous** | `RunAsync()` | Long tasks, polling or webhooks |
| **Streaming** | `Stream()` | Real-time output, interactive UIs |

### Sessions

Sessions enable **conversation continuity** — Claude remembers context across multiple interactions:

```go
// First interaction
result1, _ := client.Run(ctx, &stromboli.RunRequest{
    Prompt: "My name is Alice and I'm working on a Go project.",
})

// Continue the conversation (Claude remembers context)
result2, _ := client.Run(ctx, &stromboli.RunRequest{
    Prompt: "What's my name and what am I working on?",
    Claude: &stromboli.ClaudeOptions{
        SessionID: result1.SessionID,
        Resume:    true,
    },
})
```

## API Reference

### Client Configuration

Create a client with default settings:

```go
client := stromboli.NewClient("http://localhost:8585")
```

Configure with options:

```go
client := stromboli.NewClient("http://localhost:8585",
    stromboli.WithTimeout(5*time.Minute),     // Request timeout
    stromboli.WithRetries(3),                  // Retry on transient errors
    stromboli.WithToken("your-jwt-token"),     // Pre-set auth token
    stromboli.WithUserAgent("my-app/1.0.0"),   // Custom User-Agent
    stromboli.WithHTTPClient(customClient),    // Custom HTTP client
)
```

#### Options Reference

| Option | Description | Default |
|--------|-------------|---------|
| `WithTimeout(d)` | Request timeout | 30s |
| `WithRetries(n)` | Max retry attempts | 0 |
| `WithToken(t)` | Bearer token for auth | "" |
| `WithUserAgent(ua)` | User-Agent header | "stromboli-go/{version}" |
| `WithHTTPClient(c)` | Custom HTTP client | http.DefaultClient |

---

### Execution

#### Run (Synchronous)

Execute Claude and wait for the complete response:

```go
result, err := client.Run(ctx, &stromboli.RunRequest{
    Prompt:  "Analyze this code for bugs",
    Workdir: "/workspace",
    Claude: &stromboli.ClaudeOptions{
        Model:        stromboli.ModelSonnet,  // Model selection
        MaxBudgetUSD: 5.0,                    // Cost limit
        AllowedTools: []string{"Read", "Grep", "Glob"},
    },
    Podman: &stromboli.PodmanOptions{
        Memory:  "2g",                        // Memory limit
        Timeout: "10m",                       // Execution timeout
        Volumes: []string{"/code:/workspace:ro"},
    },
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", result.Status)
fmt.Printf("Output: %s\n", result.Output)
fmt.Printf("Session: %s\n", result.SessionID)
```

#### RunRequest Fields

| Field | Type | Description |
|-------|------|-------------|
| `Prompt` | `string` | **Required.** The prompt to send to Claude |
| `Workdir` | `string` | Working directory inside container |
| `WebhookURL` | `string` | URL for completion notification |
| `Claude` | `*ClaudeOptions` | Claude-specific configuration |
| `Podman` | `*PodmanOptions` | Container configuration |

#### ClaudeOptions

| Field | Type | Description |
|-------|------|-------------|
| `Model` | `string` | Model: `ModelSonnet`, `ModelOpus`, `ModelHaiku` |
| `SessionID` | `string` | Session ID for conversation continuity |
| `Resume` | `bool` | Resume existing session |
| `MaxBudgetUSD` | `float64` | Maximum spend in USD |
| `SystemPrompt` | `string` | Override system prompt |
| `AppendSystemPrompt` | `string` | Append to system prompt |
| `AllowedTools` | `[]string` | Whitelist of allowed tools |
| `DisallowedTools` | `[]string` | Blacklist of tools |
| `PermissionMode` | `string` | Permission mode |
| `OutputFormat` | `string` | Output format |
| `Verbose` | `bool` | Verbose output |
| `Debug` | `bool` | Debug mode |

#### PodmanOptions

| Field | Type | Description |
|-------|------|-------------|
| `Memory` | `string` | Memory limit (e.g., "512m", "2g") |
| `Timeout` | `string` | Execution timeout (e.g., "5m", "1h") |
| `Cpus` | `string` | CPU limit |
| `CPUShares` | `int64` | CPU shares |
| `Volumes` | `[]string` | Volume mounts |
| `Image` | `string` | Custom container image |
| `SecretsEnv` | `map[string]string` | Secrets to inject as env vars |

#### RunAsync (Asynchronous)

Start a long-running task and get a job ID:

```go
job, err := client.RunAsync(ctx, &stromboli.RunRequest{
    Prompt:     "Review the entire codebase for security issues",
    WebhookURL: "https://example.com/webhook", // Optional notification
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Job started: %s\n", job.JobID)

// Poll for completion
for {
    status, _ := client.GetJob(ctx, job.JobID)

    switch {
    case status.IsCompleted():
        fmt.Println(status.Output)
        return
    case status.IsFailed():
        log.Fatalf("Job failed: %s", status.Error)
    default:
        fmt.Printf("Status: %s\n", status.Status)
        time.Sleep(2 * time.Second)
    }
}
```

---

### Streaming

Stream Claude's output in real-time using Server-Sent Events (SSE):

```go
stream, err := client.Stream(ctx, &stromboli.StreamRequest{
    Prompt:    "Count from 1 to 10 slowly",
    SessionID: "optional-session-id",
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Iterator pattern
for stream.Next() {
    event := stream.Event()
    fmt.Print(event.Data) // Print as it arrives
}

if err := stream.Err(); err != nil {
    log.Fatal(err)
}
```

#### Channel-based Iteration

```go
stream, _ := client.Stream(ctx, req)
defer stream.Close()

for event := range stream.Events() {
    switch event.Type {
    case "":
        fmt.Print(event.Data) // Regular output
    case "error":
        log.Printf("Error: %s", event.Data)
    case "done":
        fmt.Println("\nStream complete")
    }
}
```

#### StreamEvent Fields

| Field | Type | Description |
|-------|------|-------------|
| `Type` | `string` | Event type ("", "error", "done") |
| `Data` | `string` | Event payload |
| `ID` | `string` | Event ID (if provided) |

---

### Jobs

#### List All Jobs

```go
jobs, err := client.ListJobs(ctx)
if err != nil {
    log.Fatal(err)
}

for _, job := range jobs {
    fmt.Printf("%s: %s (created: %s)\n",
        job.ID, job.Status, job.CreatedAt)
}
```

#### Get Job Status

```go
job, err := client.GetJob(ctx, "job-abc123def456")
if err != nil {
    if errors.Is(err, stromboli.ErrNotFound) {
        fmt.Println("Job not found")
        return
    }
    log.Fatal(err)
}

fmt.Printf("ID: %s\n", job.ID)
fmt.Printf("Status: %s\n", job.Status)
fmt.Printf("Output: %s\n", job.Output)

// Helper methods
if job.IsRunning() { fmt.Println("Still running...") }
if job.IsCompleted() { fmt.Println("Done!") }
if job.IsFailed() { fmt.Println("Failed:", job.Error) }
```

#### Cancel a Job

```go
err := client.CancelJob(ctx, "job-abc123def456")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Job cancelled")
```

#### Job Status Values

| Status | Description |
|--------|-------------|
| `pending` | Job is queued |
| `running` | Job is executing |
| `completed` | Job finished successfully |
| `failed` | Job failed with error |
| `cancelled` | Job was cancelled |

---

### Sessions

#### List Sessions

```go
sessions, err := client.ListSessions(ctx)
if err != nil {
    log.Fatal(err)
}

for _, id := range sessions {
    fmt.Printf("Session: %s\n", id)
}
```

#### Get Session Messages

Retrieve conversation history:

```go
messages, err := client.GetMessages(ctx, "sess-abc123", &stromboli.GetMessagesOptions{
    Limit:  50,
    Offset: 0,
})
if err != nil {
    log.Fatal(err)
}

for _, msg := range messages.Messages {
    fmt.Printf("[%s] %s\n", msg.Type, msg.UUID)
}

// Pagination
if messages.HasMore {
    nextPage, _ := client.GetMessages(ctx, "sess-abc123", &stromboli.GetMessagesOptions{
        Limit:  50,
        Offset: messages.Offset + messages.Limit,
    })
    // Process next page...
}
```

#### Get Single Message

```go
msg, err := client.GetMessage(ctx, "sess-abc123", "msg-uuid-456")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Type: %s\n", msg.Type)
fmt.Printf("Timestamp: %s\n", msg.Timestamp)
```

#### Destroy Session

```go
err := client.DestroySession(ctx, "sess-abc123")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Session destroyed")
```

---

### Authentication

Stromboli supports JWT-based authentication:

#### Get Token

```go
tokens, err := client.GetToken(ctx, "my-client-id")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Access Token: %s\n", tokens.AccessToken)
fmt.Printf("Expires In: %d seconds\n", tokens.ExpiresIn)

// Set token for future requests
client.SetToken(tokens.AccessToken)
```

#### Refresh Token

```go
newTokens, err := client.RefreshToken(ctx, tokens.RefreshToken)
if err != nil {
    // Refresh token expired, need to re-authenticate
    log.Fatal(err)
}

client.SetToken(newTokens.AccessToken)
```

#### Validate Token

```go
validation, err := client.ValidateToken(ctx)
if err != nil {
    log.Fatal(err)
}

if validation.Valid {
    fmt.Printf("Token valid for: %s\n", validation.Subject)
    fmt.Printf("Expires at: %d\n", validation.ExpiresAt)
}
```

#### Logout

```go
result, err := client.Logout(ctx)
if err != nil {
    log.Fatal(err)
}

if result.Success {
    fmt.Println("Logged out successfully")
    client.SetToken("") // Clear the token
}
```

---

### System

#### Health Check

```go
health, err := client.Health(ctx)
if err != nil {
    log.Fatalf("API unreachable: %v", err)
}

fmt.Printf("API: %s v%s\n", health.Name, health.Version)
fmt.Printf("Status: %s\n", health.Status)

// Check components
for _, comp := range health.Components {
    status := "✅"
    if !comp.IsHealthy() {
        status = "❌"
    }
    fmt.Printf("%s %s: %s\n", status, comp.Name, comp.Status)
}
```

#### Claude Status

```go
status, err := client.ClaudeStatus(ctx)
if err != nil {
    log.Fatal(err)
}

if status.Configured {
    fmt.Println("Claude is ready for execution")
} else {
    fmt.Printf("Claude not configured: %s\n", status.Message)
}
```

#### List Secrets

```go
secrets, err := client.ListSecrets(ctx)
if err != nil {
    log.Fatal(err)
}

for _, name := range secrets {
    fmt.Printf("Secret: %s\n", name)
}
```

---

## Version Compatibility

The SDK includes runtime version checking to ensure compatibility with the Stromboli API server.

### Quick Check

```go
health, _ := client.Health(ctx)
if !stromboli.IsCompatible(health.Version) {
    log.Printf("Warning: Server %s may not be compatible with SDK", health.Version)
}
```

### Detailed Check

```go
health, _ := client.Health(ctx)
result := stromboli.CheckCompatibility(health.Version)

switch result.Status {
case stromboli.Compatible:
    fmt.Printf("✅ Server %s is compatible\n", result.ServerVersion)
case stromboli.Incompatible:
    fmt.Printf("⚠️  %s\n", result.Message)
case stromboli.Unknown:
    fmt.Printf("❓ Could not determine: %s\n", result.Message)
}
```

### Fail Fast

```go
func main() {
    client := stromboli.NewClient(url)
    health, err := client.Health(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Panics if incompatible
    stromboli.MustBeCompatible(health.Version)

    // Continue with compatible server...
}
```

### Version Constants

| Constant | Description |
|----------|-------------|
| `stromboli.Version` | SDK version (e.g., "0.1.0") |
| `stromboli.APIVersion` | Target API version (e.g., "0.3.0-alpha") |
| `stromboli.APIVersionRange` | Supported range (e.g., ">=0.3.0-alpha <0.4.0") |

---

## Error Handling

The SDK uses typed errors for common failure cases:

```go
result, err := client.Run(ctx, req)
if err != nil {
    var apiErr *stromboli.Error
    if errors.As(err, &apiErr) {
        switch apiErr.Code {
        case "NOT_FOUND":
            fmt.Println("Resource not found")
        case "UNAUTHORIZED":
            fmt.Println("Need to authenticate")
        case "TIMEOUT":
            fmt.Println("Request timed out")
        case "BAD_REQUEST":
            fmt.Println("Invalid request:", apiErr.Message)
        default:
            fmt.Printf("API error [%s]: %s\n", apiErr.Code, apiErr.Message)
        }
    }
    return
}
```

### Error Types

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request parameters |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Access denied |
| `NOT_FOUND` | 404 | Resource not found |
| `TIMEOUT` | 408 | Request timed out |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL` | 5xx | Server error |
| `CANCELLED` | - | Request was cancelled |

### Sentinel Errors

```go
if errors.Is(err, stromboli.ErrNotFound) {
    // Handle not found
}
if errors.Is(err, stromboli.ErrTimeout) {
    // Handle timeout
}
if errors.Is(err, stromboli.ErrUnauthorized) {
    // Handle auth error
}
```

---

## Examples

### Complete Chat Application

```go
package main

import (
    "bufio"
    "context"
    "fmt"
    "os"

    "github.com/tomblancdev/stromboli-go"
)

func main() {
    client := stromboli.NewClient("http://localhost:8585")
    ctx := context.Background()

    var sessionID string
    scanner := bufio.NewScanner(os.Stdin)

    fmt.Println("Chat with Claude (type 'quit' to exit)")

    for {
        fmt.Print("\nYou: ")
        if !scanner.Scan() {
            break
        }
        input := scanner.Text()
        if input == "quit" {
            break
        }

        req := &stromboli.RunRequest{Prompt: input}
        if sessionID != "" {
            req.Claude = &stromboli.ClaudeOptions{
                SessionID: sessionID,
                Resume:    true,
            }
        }

        result, err := client.Run(ctx, req)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            continue
        }

        sessionID = result.SessionID
        fmt.Printf("\nClaude: %s\n", result.Output)
    }

    // Cleanup
    if sessionID != "" {
        client.DestroySession(ctx, sessionID)
    }
}
```

### Streaming with Progress

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/tomblancdev/stromboli-go"
)

func main() {
    client := stromboli.NewClient("http://localhost:8585")

    stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
        Prompt: "Write a short story about a robot learning to paint",
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to start stream: %v\n", err)
        os.Exit(1)
    }
    defer stream.Close()

    fmt.Println("Claude is writing...\n")

    for stream.Next() {
        event := stream.Event()
        fmt.Print(event.Data)
    }

    if err := stream.Err(); err != nil {
        fmt.Fprintf(os.Stderr, "\nStream error: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("\n\nDone!")
}
```

### Async Job with Webhook

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/tomblancdev/stromboli-go"
)

func main() {
    client := stromboli.NewClient("http://localhost:8585")
    ctx := context.Background()

    // Start async job
    job, err := client.RunAsync(ctx, &stromboli.RunRequest{
        Prompt:  "Perform a comprehensive security audit",
        Workdir: "/workspace",
        Podman: &stromboli.PodmanOptions{
            Timeout: "30m",
            Memory:  "4g",
            Volumes: []string{"/code:/workspace:ro"},
        },
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Job started: %s\n", job.JobID)

    // Poll with exponential backoff
    backoff := time.Second
    for {
        status, err := client.GetJob(ctx, job.JobID)
        if err != nil {
            panic(err)
        }

        switch {
        case status.IsCompleted():
            fmt.Printf("\n✅ Completed!\n%s\n", status.Output)
            return
        case status.IsFailed():
            fmt.Printf("\n❌ Failed: %s\n", status.Error)
            return
        default:
            fmt.Printf("⏳ %s...\n", status.Status)
            time.Sleep(backoff)
            if backoff < 30*time.Second {
                backoff *= 2
            }
        }
    }
}
```

---

## Development

### Prerequisites

- Go 1.22+
- Podman
- Make

### Commands

```bash
# Build
make build

# Run tests
make test

# Run E2E tests (requires Stromboli or Prism)
make test-e2e

# Lint
make lint

# Format
make fmt

# Generate code from OpenAPI spec
make generate
```

### Project Structure

```
stromboli-go/
├── client.go           # Main client implementation
├── types.go            # Request/response types
├── errors.go           # Error types
├── options.go          # Functional options
├── stream.go           # SSE streaming
├── version.go          # Version info
├── generated/          # Auto-generated code (don't edit)
├── tests/
│   ├── unit/           # Unit tests
│   └── e2e/            # E2E tests
└── scripts/
    └── generate.go     # Code generation script
```

### Running Tests

```bash
# Unit tests
make test

# E2E with Prism mock server
make test-e2e

# E2E with real Stromboli
STROMBOLI_URL=http://localhost:8585 STROMBOLI_REAL=1 make test-e2e
```

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Links

- [Stromboli](https://github.com/tomblancdev/stromboli) — The container orchestration server
- [Documentation](https://tomblancdev.github.io/stromboli/) — Full Stromboli documentation
- [Issues](https://github.com/tomblancdev/stromboli-go/issues) — Report bugs or request features
