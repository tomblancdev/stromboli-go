//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// TestListImages_E2E tests listing available container images.
func TestListImages_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	images, err := client.ListImages(ctx)
	require.NoError(t, err, "ListImages should succeed")

	// Log for debugging
	t.Logf("Found %d images", len(images))
	for i, img := range images {
		if i < 5 { // Only log first 5
			t.Logf("  - %s:%s (rank %d, compatible: %v)", img.Repository, img.Tag, img.CompatibilityRank, img.Compatible)
		}
	}
	if len(images) > 5 {
		t.Logf("  - ... and %d more", len(images)-5)
	}
}

// TestGetImage_E2E tests getting a specific image.
//
// Skip for mock server as Prism may not handle path params correctly.
func TestGetImage_E2E(t *testing.T) {
	skipIfMock(t, "Prism may not handle path params correctly")

	client := newTestClient()
	ctx := newTestContext(t)

	// First list images to get a valid name
	images, err := client.ListImages(ctx)
	require.NoError(t, err, "ListImages should succeed")
	require.NotEmpty(t, images, "Should have at least one image")

	// Get the first image
	imageName := images[0].Repository
	if images[0].Tag != "" {
		imageName += ":" + images[0].Tag
	}

	image, err := client.GetImage(ctx, imageName)
	require.NoError(t, err, "GetImage should succeed")
	require.NotNil(t, image, "Image should not be nil")

	t.Logf("Image: %s", image.ID)
	t.Logf("Repository: %s", image.Repository)
	t.Logf("Tag: %s", image.Tag)
	t.Logf("Compatible: %v (rank %d)", image.Compatible, image.CompatibilityRank)
}

// TestSearchImages_E2E tests searching for images.
func TestSearchImages_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	results, err := client.SearchImages(ctx, &stromboli.SearchImagesOptions{
		Query: "python",
		Limit: 5,
	})
	require.NoError(t, err, "SearchImages should succeed")

	// Log for debugging
	t.Logf("Found %d results for 'python'", len(results))
	for _, r := range results {
		t.Logf("  - %s: %s (stars: %d, official: %v)", r.Name, r.Description, r.Stars, r.Official)
	}
}

// TestSearchImages_EmptyQuery_E2E tests that empty query returns error.
func TestSearchImages_EmptyQuery_E2E(t *testing.T) {
	client := newTestClient()
	ctx := newTestContext(t)

	_, err := client.SearchImages(ctx, &stromboli.SearchImagesOptions{
		Query: "",
	})
	require.Error(t, err, "SearchImages with empty query should fail")

	var apiErr *stromboli.Error
	require.ErrorAs(t, err, &apiErr, "Should be an API error")
	assert.Equal(t, "BAD_REQUEST", apiErr.Code)
}

// TestPullImage_E2E tests pulling an image.
//
// Skip by default as this can take a long time and requires network access.
func TestPullImage_E2E(t *testing.T) {
	skipIfMock(t, "PullImage requires real Podman")

	if testing.Short() {
		t.Skip("Skipping PullImage in short mode (can be slow)")
	}

	client := newTestClient()
	ctx := newTestContext(t)

	result, err := client.PullImage(ctx, &stromboli.PullImageRequest{
		Image: "docker.io/library/alpine:latest",
		Quiet: true,
	})
	require.NoError(t, err, "PullImage should succeed")
	require.True(t, result.Success, "Pull should be successful")
	require.NotEmpty(t, result.ImageID, "Should return image ID")

	t.Logf("Pulled image: %s (ID: %s)", result.Image, result.ImageID)
}
