package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tomblancdev/stromboli-go"
)

// TestVersion_Constants verifies version constants are set correctly.
func TestVersion_Constants(t *testing.T) {
	// Verify constants are not empty
	assert.NotEmpty(t, stromboli.Version, "Version should not be empty")
	assert.NotEmpty(t, stromboli.APIVersion, "APIVersion should not be empty")
	assert.NotEmpty(t, stromboli.APIVersionRange, "APIVersionRange should not be empty")

	// Log values for debugging
	t.Logf("SDK Version: %s", stromboli.Version)
	t.Logf("API Version: %s", stromboli.APIVersion)
	t.Logf("API Range: %s", stromboli.APIVersionRange)
}

// TestIsCompatible tests the IsCompatible convenience function.
func TestIsCompatible(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		compatible bool
	}{
		{
			name:       "exact target version",
			version:    "0.3.0-alpha",
			compatible: true,
		},
		{
			name:       "patch version in range",
			version:    "0.3.1",
			compatible: true,
		},
		{
			name:       "minor version in range",
			version:    "0.3.5",
			compatible: true,
		},
		{
			name:       "version too old",
			version:    "0.2.0",
			compatible: false,
		},
		{
			name:       "version too new",
			version:    "0.4.0",
			compatible: false,
		},
		{
			name:       "major version mismatch",
			version:    "1.0.0",
			compatible: false,
		},
		{
			name:       "empty version",
			version:    "",
			compatible: false,
		},
		{
			name:       "invalid version",
			version:    "not-a-version",
			compatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stromboli.IsCompatible(tt.version)
			assert.Equal(t, tt.compatible, result, "IsCompatible(%q) should return %v", tt.version, tt.compatible)
		})
	}
}

// TestCheckCompatibility_Compatible tests CheckCompatibility with compatible versions.
func TestCheckCompatibility_Compatible(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"exact version", "0.3.0-alpha"},
		{"patch version", "0.3.1"},
		{"minor patch", "0.3.99"},
		{"with prerelease", "0.3.1-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stromboli.CheckCompatibility(tt.version)

			assert.Equal(t, stromboli.Compatible, result.Status)
			assert.True(t, result.IsCompatible())
			assert.Equal(t, tt.version, result.ServerVersion)
			assert.Equal(t, stromboli.Version, result.SDKVersion)
			assert.Equal(t, stromboli.APIVersion, result.TargetAPIVersion)
			assert.Equal(t, stromboli.APIVersionRange, result.SupportedRange)
			assert.Contains(t, result.Message, "compatible")
		})
	}
}

// TestCheckCompatibility_Incompatible tests CheckCompatibility with incompatible versions.
func TestCheckCompatibility_Incompatible(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"too old", "0.2.0"},
		{"too new minor", "0.4.0"},
		{"too new major", "1.0.0"},
		{"way too old", "0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stromboli.CheckCompatibility(tt.version)

			assert.Equal(t, stromboli.Incompatible, result.Status)
			assert.False(t, result.IsCompatible())
			assert.Equal(t, tt.version, result.ServerVersion)
			assert.Contains(t, result.Message, "not compatible")
		})
	}
}

// TestCheckCompatibility_Unknown tests CheckCompatibility with unparseable versions.
func TestCheckCompatibility_Unknown(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"empty string", ""},
		{"invalid format", "not-a-version"},
		{"garbage", "abc.def.ghi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stromboli.CheckCompatibility(tt.version)

			assert.Equal(t, stromboli.Unknown, result.Status)
			assert.False(t, result.IsCompatible())
			assert.NotEmpty(t, result.Message)
		})
	}
}

// TestCompatibilityStatus_String tests the String method on CompatibilityStatus.
func TestCompatibilityStatus_String(t *testing.T) {
	tests := []struct {
		status   stromboli.CompatibilityStatus
		expected string
	}{
		{stromboli.Compatible, "compatible"},
		{stromboli.Incompatible, "incompatible"},
		{stromboli.Unknown, "unknown"},
		{stromboli.CompatibilityStatus(99), "unknown"}, // Invalid value defaults to unknown
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

// TestMustBeCompatible_Compatible tests MustBeCompatible doesn't panic with compatible version.
func TestMustBeCompatible_Compatible(t *testing.T) {
	// Should not panic
	require.NotPanics(t, func() {
		stromboli.MustBeCompatible("0.3.0-alpha")
	})
}

// TestMustBeCompatible_Incompatible tests MustBeCompatible panics with incompatible version.
func TestMustBeCompatible_Incompatible(t *testing.T) {
	// Should panic
	require.Panics(t, func() {
		stromboli.MustBeCompatible("0.1.0")
	})
}

// TestMustBeCompatible_Invalid tests MustBeCompatible panics with invalid version.
func TestMustBeCompatible_Invalid(t *testing.T) {
	// Should panic
	require.Panics(t, func() {
		stromboli.MustBeCompatible("invalid")
	})
}

// TestCompatibilityResult_Fields tests all fields are populated correctly.
func TestCompatibilityResult_Fields(t *testing.T) {
	result := stromboli.CheckCompatibility("0.3.0-alpha")

	assert.Equal(t, "0.3.0-alpha", result.ServerVersion)
	assert.Equal(t, stromboli.Version, result.SDKVersion)
	assert.Equal(t, stromboli.APIVersion, result.TargetAPIVersion)
	assert.Equal(t, stromboli.APIVersionRange, result.SupportedRange)
	assert.NotEmpty(t, result.Message)
}
