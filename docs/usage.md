# Usage Guide

## Installation

```bash
go get github.com/tomblancdev/stromboli-go
```

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
    // Create client
    client := stromboli.NewClient("http://localhost:8585")

    // Run a prompt
    result, err := client.Run(context.Background(), &stromboli.RunRequest{
        Prompt: "Hello, Claude!",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Output)
}
```

## Client Options

```go
client := stromboli.NewClient("http://localhost:8585",
    // Request timeout (default: 30s)
    stromboli.WithTimeout(5*time.Minute),

    // Retry failed requests (default: 0)
    stromboli.WithRetries(3),

    // Custom HTTP client
    stromboli.WithHTTPClient(&http.Client{
        Transport: customTransport,
    }),
)
```

## Synchronous Execution

```go
result, err := client.Run(ctx, &stromboli.RunRequest{
    Prompt:    "Explain quantum computing",
    Model:     "haiku",           // Optional: haiku, sonnet, opus
    Workspace: "/path/to/code",   // Optional: mount directory
})
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Output)
fmt.Println(result.SessionID)  // For session continuation
```

## Async Execution

```go
// Start async job
job, err := client.RunAsync(ctx, &stromboli.RunRequest{
    Prompt: "Review this large codebase",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Job started: %s\n", job.ID)

// Poll for completion
for {
    status, err := client.GetJob(ctx, job.ID)
    if err != nil {
        log.Fatal(err)
    }

    switch status.Status {
    case "completed":
        fmt.Println(status.Output)
        return
    case "failed":
        log.Fatalf("Job failed: %s", status.Error)
    default:
        time.Sleep(time.Second)
    }
}
```

## Session Continuation

```go
// First request
result1, _ := client.Run(ctx, &stromboli.RunRequest{
    Prompt: "Remember: my name is Tom",
})

// Continue session
result2, _ := client.Run(ctx, &stromboli.RunRequest{
    Prompt:    "What's my name?",
    SessionID: result1.SessionID,  // Continue conversation
})
```

## List Jobs

```go
jobs, err := client.ListJobs(ctx)
if err != nil {
    log.Fatal(err)
}

for _, job := range jobs.Jobs {
    fmt.Printf("%s: %s\n", job.ID, job.Status)
}
```

## Cancel Job

```go
err := client.CancelJob(ctx, "job-abc123")
if err != nil {
    log.Fatal(err)
}
```

## Health Check

```go
health, err := client.Health(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", health.Status)
fmt.Printf("Version: %s\n", health.Version)

for _, c := range health.Components {
    fmt.Printf("  %s: %s\n", c.Name, c.Status)
}
```

## Error Handling

```go
result, err := client.Run(ctx, req)
if err != nil {
    var stromboliErr *stromboli.Error
    if errors.As(err, &stromboliErr) {
        switch stromboliErr.Code {
        case "NOT_FOUND":
            // Handle not found
        case "TIMEOUT":
            // Handle timeout
        case "UNAUTHORIZED":
            // Handle auth error
        default:
            log.Printf("API error: %s", stromboliErr.Message)
        }
    } else {
        // Network or other error
        log.Fatal(err)
    }
}
```

## Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := client.Run(ctx, req)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Request timed out")
}
```
