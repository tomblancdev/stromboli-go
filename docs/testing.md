# Testing Guide

This document explains how to test the Stromboli Go SDK.

## Test Strategy

The SDK uses a two-tier testing approach:

| Tier | Type | Purpose | Speed |
|------|------|---------|-------|
| **Unit** | HTTP mock | Test wrapper logic, error handling | Fast |
| **E2E** | Real/Prism | Test full integration | Slower |

## Unit Tests

Unit tests use `httptest.Server` to mock HTTP responses. This approach:
- Tests the full wrapper layer (client.go → generated → HTTP)
- Validates request construction
- Validates response parsing
- Tests error handling

### Running Unit Tests

```bash
make test
```

### Example Unit Test

```go
func TestHealth_Success(t *testing.T) {
    // Arrange: Create mock server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request
        assert.Equal(t, "/health", r.URL.Path)

        // Return mock response
        json.NewEncoder(w).Encode(map[string]interface{}{
            "name":    "stromboli",
            "status":  "ok",
            "version": "0.3.0-alpha",
        })
    }))
    defer server.Close()

    // Act
    client := stromboli.NewClient(server.URL)
    health, err := client.Health(context.Background())

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "ok", health.Status)
}
```

### Test Patterns

#### Testing Success Cases
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(successResponse)
}))
```

#### Testing Error Responses
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNotFound)
    json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
}))
```

#### Testing Context Cancellation
```go
ctx, cancel := context.WithCancel(context.Background())
cancel() // Cancel immediately
_, err := client.Health(ctx)
require.Error(t, err)
```

## Interface Mocks (mockery)

For testing components that depend on the generated client interfaces:

### Generate Mocks

```bash
make mocks
```

This creates mocks in `mocks/` directory.

### Using Mocks

```go
import "github.com/tomblancdev/stromboli-go/mocks"

func TestSomething(t *testing.T) {
    mockClient := mocks.NewClientService(t)

    mockClient.EXPECT().
        GetHealth(mock.Anything).
        Return(&system.GetHealthOK{...}, nil)

    // Use mockClient in your test
}
```

## E2E Tests

E2E tests run against a real Stromboli instance or a mock server.

### Option 1: Prism Mock Server

Prism is an OpenAPI mock server that returns valid responses based on the schema.

```bash
# Terminal 1: Start mock server
make mock-server
# Runs on http://localhost:4010

# Terminal 2: Run E2E tests
make test-e2e
```

### Option 2: Real Stromboli

```bash
# Start Stromboli (separate terminal)
stromboli serve

# Run E2E tests
STROMBOLI_URL=http://localhost:8585 make test-e2e
```

### E2E Test Structure

E2E tests live in `tests/e2e/` and use build tags:

```go
//go:build e2e

package e2e

import (
    "os"
    "testing"
)

func getBaseURL() string {
    if url := os.Getenv("STROMBOLI_URL"); url != "" {
        return url
    }
    return "http://localhost:4010" // Prism default
}

func TestHealthE2E(t *testing.T) {
    client := stromboli.NewClient(getBaseURL())
    health, err := client.Health(context.Background())
    require.NoError(t, err)
    assert.Equal(t, "ok", health.Status)
}
```

## Coverage

Generate coverage report:

```bash
make test-coverage
# Opens coverage.html
```

Target: **80%+ coverage** on wrapper code.

## Test Organization

```
tests/
├── unit/
│   ├── client_test.go      # Client wrapper tests
│   ├── health_test.go      # Health method tests
│   ├── execution_test.go   # Execution method tests
│   └── ...
└── e2e/
    ├── smoke_test.go       # Basic connectivity
    ├── health_test.go      # Health endpoint E2E
    └── ...
```

## CI/CD Integration

```yaml
test:
  steps:
    - make lint
    - make test
    - make test-coverage

e2e:
  services:
    - prism mock generated/swagger.yaml --host 0.0.0.0 --port 4010
  steps:
    - make test-e2e
```
