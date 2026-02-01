// Package stromboli provides a Go SDK for the Stromboli API.
//
// Stromboli is a container orchestration service for Claude Code agents,
// enabling isolated execution of Claude prompts in Podman containers.
//
// Basic usage:
//
//	client := stromboli.NewClient("http://localhost:8585")
//	result, err := client.Run(ctx, &stromboli.RunRequest{
//	    Prompt: "Hello!",
//	})
package stromboli
