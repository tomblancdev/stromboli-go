//go:build ignore

// Package stromboli_test contains example code for the Stromboli Go SDK.
//
// These examples are intended for documentation purposes and demonstrate
// how to use the SDK. They are NOT run as tests because they require
// a running Stromboli server.
//
// To include these in godoc, they are provided as compilable examples
// but excluded from test runs with the `ignore` build tag.
package stromboli_test

import (
	"context"
	"fmt"
	"log"

	"github.com/tomblancdev/stromboli-go"
)

// This example shows how to check API health.
// Note: This example requires a running Stromboli server.
func Example_health() {
	// Create a new client
	client, err := stromboli.NewClient("http://localhost:8585")
	if err != nil {
		log.Fatal(err)
	}

	// Check API health
	health, err := client.Health(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Status: %s, Version: %s\n", health.Status, health.Version)
}

func Example_run() {
	// Create a new client
	client, err := stromboli.NewClient("http://localhost:8585")
	if err != nil {
		log.Fatal(err)
	}

	// Execute Claude synchronously
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Say hello in exactly 2 words",
	})
	if err != nil {
		log.Fatal(err)
	}

	if result.IsSuccess() {
		fmt.Println("Response received successfully")
	}
}

func ExampleNewClient() {
	// Basic client creation
	client, err := stromboli.NewClient("http://localhost:8585")
	if err != nil {
		log.Fatal(err)
	}
	_ = client // Use client
}

func ExampleNewClient_withOptions() {
	// Client with custom options
	client, err := stromboli.NewClient("http://localhost:8585",
		stromboli.WithTimeout(5*60*1000*1000*1000), // 5 minutes in nanoseconds
		stromboli.WithUserAgent("my-app/1.0.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client // Use client
}

func ExampleClient_Run() {
	client, _ := stromboli.NewClient("http://localhost:8585")

	// Simple execution
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt: "Hello, Claude!",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Output)
}

func ExampleClient_Run_withOptions() {
	client, _ := stromboli.NewClient("http://localhost:8585")

	// Execution with full configuration
	result, err := client.Run(context.Background(), &stromboli.RunRequest{
		Prompt:  "Review this code for security issues",
		Workdir: "/workspace",
		Claude: &stromboli.ClaudeOptions{
			Model:        stromboli.ModelSonnet,
			MaxBudgetUSD: 5.0,
			AllowedTools: []string{"Read", "Glob", "Grep"},
		},
		Podman: &stromboli.PodmanOptions{
			Memory:  "2g",
			Timeout: "10m",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	if result.IsSuccess() {
		fmt.Println("Review completed")
	}
}

func ExampleClient_RunAsync() {
	client, _ := stromboli.NewClient("http://localhost:8585")

	// Start async execution
	async, err := client.RunAsync(context.Background(), &stromboli.RunRequest{
		Prompt: "Analyze this large codebase",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Job started: %s\n", async.JobID)

	// Poll for completion (simplified - in production use proper polling)
	job, _ := client.GetJob(context.Background(), async.JobID)
	if job.IsCompleted() {
		fmt.Println(job.Output)
	}
}

func ExampleClient_Stream() {
	client, _ := stromboli.NewClient("http://localhost:8585")

	// Create a streaming request
	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "Count from 1 to 5",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	// Iterate over events
	for stream.Next() {
		event := stream.Event()
		fmt.Print(event.Data)
	}

	if err := stream.Err(); err != nil {
		log.Print(err)
	}
}

func ExampleStream_EventsWithContext() {
	client, _ := stromboli.NewClient("http://localhost:8585")

	stream, err := client.Stream(context.Background(), &stromboli.StreamRequest{
		Prompt: "Count from 1 to 10",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	// Use EventsWithContext to avoid goroutine leaks if you stop early
	ctx, cancel := context.WithCancel(context.Background())

	for event := range stream.EventsWithContext(ctx) {
		fmt.Print(event.Data)
		// Can safely cancel ctx here and the goroutine will exit
	}
	cancel() // Clean up the context
}

func ExampleClient_GetToken() {
	client, _ := stromboli.NewClient("http://localhost:8585")

	// Obtain JWT tokens
	tokens, err := client.GetToken(context.Background(), "my-client-id")
	if err != nil {
		log.Fatal(err)
	}

	// Set token for authenticated requests
	client.SetToken(tokens.AccessToken)

	fmt.Printf("Token type: %s, Expires in: %d seconds\n",
		tokens.TokenType, tokens.ExpiresIn)
}

func ExampleClient_ListSecrets() {
	client, _ := stromboli.NewClient("http://localhost:8585")

	// List available Podman secrets
	secrets, err := client.ListSecrets(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, s := range secrets {
		fmt.Printf("Secret: %s (created: %s)\n", s.Name, s.CreatedAt)
	}
}

func ExampleModel() {
	// Model type provides type-safe model selection
	model := stromboli.ModelSonnet
	fmt.Println(model.String())
	// Output: sonnet
}
