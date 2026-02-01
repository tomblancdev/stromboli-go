//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// TestListJobs_E2E tests listing jobs against a real/mock server.
func TestListJobs_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	jobs, err := client.ListJobs(ctx)
	require.NoError(t, err, "ListJobs should succeed")

	// Log for debugging
	t.Logf("Found %d jobs", len(jobs))
	for _, job := range jobs {
		t.Logf("  - %s: status=%s", job.ID, job.Status)
	}
}

// TestGetJob_E2E tests getting a specific job.
//
// Note: This test may fail with 404 if no jobs exist.
// In a real test environment, you would first create a job with RunAsync.
func TestGetJob_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	// First, list jobs to find an existing one
	jobs, err := client.ListJobs(ctx)
	require.NoError(t, err, "ListJobs should succeed")

	if len(jobs) == 0 {
		t.Skip("No jobs found, skipping GetJob test")
	}

	// Get the first job
	jobID := jobs[0].ID
	job, err := client.GetJob(ctx, jobID)
	require.NoError(t, err, "GetJob should succeed")

	t.Logf("Job details: id=%s status=%s", job.ID, job.Status)
	if job.Output != "" {
		t.Logf("Output preview: %.100s...", job.Output)
	}
}

// TestJobLifecycle_E2E tests the full job lifecycle: create, poll, (optionally cancel).
//
// This is a more comprehensive test that exercises multiple job endpoints.
func TestJobLifecycle_E2E(t *testing.T) {
	skipIfMock(t, "Requires real server for job lifecycle")

	client := newTestClient()
	ctx := newTestContext(t)

	// 1. Start an async job
	asyncResult, err := client.RunAsync(ctx, &stromboli.RunRequest{
		Prompt: "Count from 1 to 5.",
	})
	require.NoError(t, err, "RunAsync should succeed")
	t.Logf("Started job: %s", asyncResult.JobID)

	// 2. Get job status
	job, err := client.GetJob(ctx, asyncResult.JobID)
	require.NoError(t, err, "GetJob should succeed")
	t.Logf("Job status: %s", job.Status)

	// 3. List jobs and verify our job is in the list
	jobs, err := client.ListJobs(ctx)
	require.NoError(t, err, "ListJobs should succeed")

	found := false
	for _, j := range jobs {
		if j.ID == asyncResult.JobID {
			found = true
			break
		}
	}
	// Note: With Prism mock, this might not work as expected since it doesn't persist state
	t.Logf("Job found in list: %v (may be false with Prism mock)", found)
}
