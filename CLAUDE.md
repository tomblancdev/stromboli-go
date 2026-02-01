# Stromboli Go SDK - Claude Guidelines 🌋

## Overview

Official Go SDK for [Stromboli](https://github.com/tomblancdev/stromboli) - Container orchestration for Claude Code agents.

## Versioning

| What | Where | Example |
|------|-------|---------|
| SDK version | `version.go` | `1.0.0` |
| Target API version | `stromboli.yaml` → `apiVersion` | `0.2.0` |
| Compatible range | `stromboli.yaml` → `apiVersionRange` | `>=0.2.0 <0.3.0` |

**OpenAPI Source** (derived from `apiVersion`):
```
https://raw.githubusercontent.com/tomblancdev/stromboli/v{apiVersion}/docs/swagger/swagger.yaml
```

## Architecture

```
stromboli-go/
├── generated/               # 🤖 AUTO-GENERATED (never edit)
│   ├── types.gen.go         # OpenAPI → Go types
│   └── client.gen.go        # Generated HTTP client
├── client.go                # High-level wrapper
├── errors.go                # Custom error types
├── options.go               # Functional options
├── stromboli.go             # Main package exports
├── version.go               # SDK version
├── stromboli.yaml           # Config (API version, etc.)
├── tests/
│   ├── unit/                # Unit tests
│   └── e2e/                 # E2E tests
├── scripts/
│   └── generate.go          # Codegen script
├── Containerfile            # Dev container
├── Makefile                 # All commands
├── .golangci.yml            # Linter config
└── go.mod
```

## Development Rules

### 🐳 Always Use Containers

**NEVER run commands directly on the host.** All development happens in Podman.

```bash
# ✅ Correct
make test
make lint
make build

# ❌ Wrong
go test ./...
golangci-lint run
```

### 🤖 Auto-Generation First

**NEVER manually write API types or endpoints.**

```bash
make generate  # Fetches swagger.yaml → generates generated/*.gen.go
```

The `generate` script:
1. Reads `apiVersion` from `stromboli.yaml`
2. Fetches `swagger.yaml` from the tagged release
3. Runs `oapi-codegen` → outputs to `generated/`

**If the API changes → regenerate. Don't patch manually.**

### 🧪 Test-Driven Development

```
RED    → Write failing test
GREEN  → Minimal code to pass
REFACTOR → Clean up
```

```bash
make test           # Unit tests
make test-e2e       # E2E tests (real Stromboli)
make test-coverage  # Target: 80%+
```

### 🎯 KISS - Keep It Simple

Two layers only:

| Layer | Location | Purpose |
|-------|----------|---------|
| Generated | `generated/` | Raw, type-safe API calls |
| Client | `client.go` | Thin wrapper (retries, streaming, errors) |

**Prefer stdlib.** Only add dependencies when truly needed.

## Tech Stack

| Component | Tool | Why |
|-----------|------|-----|
| Language | Go 1.22+ | Latest stable |
| Codegen | oapi-codegen | Industry standard for Go |
| HTTP Client | net/http | Stdlib, no deps |
| Linter | golangci-lint | Meta-linter, comprehensive |
| Testing | testing + testify | Stdlib + assertions |
| Container | Podman | Rootless, daemonless |

## Makefile Commands

```bash
# Development
make dev            # Run with hot reload
make shell          # Container shell

# Code Quality
make lint           # Run golangci-lint
make fmt            # Format code
make vet            # Go vet

# Testing
make test           # Unit tests
make test-e2e       # E2E (needs Stromboli)
make test-race      # Race detector
make test-coverage  # Coverage report

# Build & Generate
make build          # Build binary
make generate       # Regenerate from OpenAPI

# Release
make release        # Tag and push release
make docs           # Generate godoc
```

## Code Standards

### Package Structure

```go
// stromboli.go - Package docs and main exports
package stromboli

// Client is the main Stromboli API client.
type Client struct { ... }

// Run executes a Claude prompt synchronously.
func (c *Client) Run(ctx context.Context, req *RunRequest) (*RunResponse, error)
```

### Naming

| Type | Convention | Example |
|------|------------|---------|
| Package | lowercase | `stromboli` |
| Exported | PascalCase | `Client`, `RunRequest` |
| Unexported | camelCase | `doRequest`, `parseError` |
| Files | snake_case | `client_test.go` |

### Error Handling

```go
// Custom error type
type Error struct {
    Code    string
    Message string
    Status  int
    Cause   error
}

func (e *Error) Error() string {
    return fmt.Sprintf("stromboli: %s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
    return e.Cause
}

// Sentinel errors
var (
    ErrNotFound     = &Error{Code: "NOT_FOUND", Message: "resource not found"}
    ErrTimeout      = &Error{Code: "TIMEOUT", Message: "request timed out"}
    ErrUnauthorized = &Error{Code: "UNAUTHORIZED", Message: "invalid credentials"}
)
```

### Context

```go
// Always accept context as first parameter
func (c *Client) Run(ctx context.Context, req *RunRequest) (*RunResponse, error)

// Use context for cancellation and timeouts
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

### Functional Options

```go
// Options pattern for flexible configuration
type Option func(*Client)

func WithTimeout(d time.Duration) Option {
    return func(c *Client) {
        c.timeout = d
    }
}

func WithRetries(n int) Option {
    return func(c *Client) {
        c.maxRetries = n
    }
}

// Usage
client := stromboli.NewClient(baseURL,
    stromboli.WithTimeout(30*time.Second),
    stromboli.WithRetries(3),
)
```

### Interfaces

```go
// Define interfaces where used, keep them small
type HTTPClient interface {
    Do(*http.Request) (*http.Response, error)
}

// Accept interfaces, return structs
func NewClient(baseURL string, opts ...Option) *Client
```

## Containerfile

```dockerfile
FROM docker.io/golang:1.22-alpine

WORKDIR /app

# Install tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["go", "test", "./..."]
```

## CI/CD Pipeline

### On Push/PR

```yaml
test:
  - make lint
  - make test
  - make build

e2e:
  - Start Stromboli container
  - make test-e2e
```

### On Stromboli Release (webhook/cron)

```yaml
sync-api:
  - Update apiVersion in stromboli.yaml
  - make generate
  - make test
  - Create PR: "chore: sync with Stromboli vX.Y.Z"
```

### On SDK Release

```yaml
release:
  - make test
  - git tag vX.Y.Z
  - git push --tags
  - Generate release notes
```

## Compatibility

Maintained in `COMPATIBILITY.md`:

| SDK Version | Stromboli API | Status |
|-------------|---------------|--------|
| 1.0.x       | 0.2.x         | ✅ Current |
| 0.9.x       | 0.1.x         | ⚠️ Deprecated |

Runtime check:

```go
health, _ := client.Health(ctx)
if !IsCompatible(health.Version) {
    log.Printf("Warning: API %s may not be compatible", health.Version)
}
```

## Usage Examples

### Basic

```go
package main

import (
    "context"
    "fmt"
    "github.com/tomblancdev/stromboli-go"
)

func main() {
    client := stromboli.NewClient("http://localhost:8585")

    result, err := client.Run(context.Background(), &stromboli.RunRequest{
        Prompt: "Hello!",
        Model:  stromboli.ModelHaiku,
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(result.Output)
}
```

### Async Job

```go
job, _ := client.RunAsync(ctx, &stromboli.RunRequest{
    Prompt:  "Review this code",
    Workdir: "/workspace",
})

// Poll for completion
for {
    status, _ := client.GetJob(ctx, job.ID)
    if status.Status == "completed" {
        fmt.Println(status.Output)
        break
    }
    time.Sleep(time.Second)
}
```

### Streaming

```go
stream, _ := client.Stream(ctx, &stromboli.RunRequest{
    Prompt: "Count to 10",
})

for event := range stream.Events() {
    fmt.Print(event.Data)
}
```

### With Options

```go
client := stromboli.NewClient(baseURL,
    stromboli.WithTimeout(5*time.Minute),
    stromboli.WithRetries(3),
    stromboli.WithHTTPClient(customHTTPClient),
)
```

## References

- [Stromboli Docs](https://tomblancdev.github.io/stromboli/)
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
