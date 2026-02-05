// Package stromboli provides a Go SDK for the Stromboli API.
//
// Stromboli is a container orchestration service for Claude Code agents,
// enabling isolated execution of Claude prompts in Podman containers.
// This SDK provides a clean, idiomatic Go interface to interact with
// the Stromboli API.
//
// # Installation
//
// To install the SDK, use go get:
//
//	go get github.com/tomblancdev/stromboli-go
//
// # Quick Start
//
// Create a client and execute a prompt:
//
//	package main
//
//	import (
//	    "context"
//	    "fmt"
//	    "log"
//
//	    "github.com/tomblancdev/stromboli-go"
//	)
//
//	func main() {
//	    // Create a new client
//	    client, err := stromboli.NewClient("http://localhost:8585")
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Check API health
//	    health, err := client.Health(context.Background())
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Printf("API Status: %s (v%s)\n", health.Status, health.Version)
//	}
//
// # Client Configuration
//
// The client can be configured using functional options:
//
//	client, err := stromboli.NewClient("http://localhost:8585",
//	    stromboli.WithTimeout(5*time.Minute),
//	    stromboli.WithHTTPClient(customHTTPClient),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Error Handling
//
// The SDK provides typed errors for common failure cases:
//
//	result, err := client.Run(ctx, req)
//	if err != nil {
//	    var apiErr *stromboli.Error
//	    if errors.As(err, &apiErr) {
//	        switch apiErr.Code {
//	        case "NOT_FOUND":
//	            // Handle not found
//	        case "TIMEOUT":
//	            // Handle timeout
//	        }
//	    }
//	}
//
// # Architecture
//
// The SDK is built in two layers:
//
//   - Wrapper Layer: Clean, idiomatic Go API with context support,
//     retries, and error handling (this package)
//   - Generated Layer: Auto-generated HTTP client from OpenAPI spec
//     (github.com/tomblancdev/stromboli-go/generated)
//
// Users should only interact with the wrapper layer. The generated
// layer is an implementation detail and may change between versions.
//
// # Thread Safety
//
// The [Client] is safe for concurrent use by multiple goroutines.
// Each method call is independent and does not share state.
//
// # API Version Compatibility
//
// This SDK version targets Stromboli API v0.4.0-alpha. Compatibility
// with other API versions is not guaranteed. Use [Client.Health] to
// check the server version at runtime.
package stromboli
