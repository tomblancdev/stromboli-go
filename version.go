package stromboli

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// Version is the current SDK version.
//
// This version follows semantic versioning (https://semver.org/).
// The version is incremented according to the following rules:
//   - MAJOR: Breaking changes to the public API
//   - MINOR: New features, backwards compatible
//   - PATCH: Bug fixes, backwards compatible
const Version = "0.1.0-alpha"

// APIVersion is the target Stromboli API version this SDK was built for.
//
// The SDK is tested against this API version and may not work correctly
// with significantly different API versions. Use [Client.Health] to check
// the actual server version at runtime.
const APIVersion = "0.3.0-alpha"

// APIVersionRange defines the range of Stromboli API versions this SDK
// is compatible with, using semver constraint syntax.
//
// Examples of constraint syntax:
//   - ">=1.0.0 <2.0.0" — versions 1.x.x
//   - "^1.2.3" — versions >=1.2.3 and <2.0.0
//   - "~1.2.3" — versions >=1.2.3 and <1.3.0
//
// Use [IsCompatible] or [CheckCompatibility] to verify a server version.
const APIVersionRange = ">=0.3.0-alpha <0.4.0"

// CompatibilityStatus represents the result of a version compatibility check.
type CompatibilityStatus int

const (
	// Compatible means the API version is within the supported range.
	Compatible CompatibilityStatus = iota

	// Incompatible means the API version is outside the supported range.
	Incompatible

	// Unknown means the version could not be parsed.
	Unknown
)

// String returns a human-readable representation of the status.
func (s CompatibilityStatus) String() string {
	switch s {
	case Compatible:
		return "compatible"
	case Incompatible:
		return "incompatible"
	default:
		return "unknown"
	}
}

// CompatibilityResult contains detailed information about API compatibility.
type CompatibilityResult struct {
	// Status is the compatibility status.
	Status CompatibilityStatus

	// ServerVersion is the version reported by the server.
	ServerVersion string

	// SDKVersion is this SDK's version.
	SDKVersion string

	// TargetAPIVersion is the API version this SDK was built for.
	TargetAPIVersion string

	// SupportedRange is the range of API versions this SDK supports.
	SupportedRange string

	// Message is a human-readable description of the result.
	Message string
}

// IsCompatible returns true if the status indicates compatibility.
func (r *CompatibilityResult) IsCompatible() bool {
	return r.Status == Compatible
}

// IsCompatible checks if a server version is compatible with this SDK.
//
// This is a convenience function that returns true if the version falls
// within [APIVersionRange]. Use [CheckCompatibility] for detailed results.
//
// Example:
//
//	health, _ := client.Health(ctx)
//	if !stromboli.IsCompatible(health.Version) {
//	    log.Printf("Warning: API %s may not be compatible", health.Version)
//	}
//
// Returns false if the version string cannot be parsed.
func IsCompatible(serverVersion string) bool {
	result := CheckCompatibility(serverVersion)
	return result.Status == Compatible
}

// CheckCompatibility performs a detailed compatibility check between
// the server version and this SDK.
//
// Example:
//
//	health, _ := client.Health(ctx)
//	result := stromboli.CheckCompatibility(health.Version)
//
//	switch result.Status {
//	case stromboli.Compatible:
//	    fmt.Println("Server is compatible")
//	case stromboli.Incompatible:
//	    fmt.Printf("Warning: %s\n", result.Message)
//	case stromboli.Unknown:
//	    fmt.Printf("Could not determine compatibility: %s\n", result.Message)
//	}
func CheckCompatibility(serverVersion string) *CompatibilityResult {
	result := &CompatibilityResult{
		ServerVersion:    serverVersion,
		SDKVersion:       Version,
		TargetAPIVersion: APIVersion,
		SupportedRange:   APIVersionRange,
	}

	// Handle empty version
	if serverVersion == "" {
		result.Status = Unknown
		result.Message = "server version is empty"
		return result
	}

	// Parse the server version
	sv, err := semver.NewVersion(serverVersion)
	if err != nil {
		result.Status = Unknown
		result.Message = fmt.Sprintf("could not parse server version %q: %v", serverVersion, err)
		return result
	}

	// Parse the constraint
	constraint, err := semver.NewConstraint(APIVersionRange)
	if err != nil {
		result.Status = Unknown
		result.Message = fmt.Sprintf("invalid SDK version constraint %q: %v", APIVersionRange, err)
		return result
	}

	// Check compatibility
	if constraint.Check(sv) {
		result.Status = Compatible
		result.Message = fmt.Sprintf("server version %s is compatible with SDK (supports %s)",
			serverVersion, APIVersionRange)
	} else {
		result.Status = Incompatible
		result.Message = fmt.Sprintf("server version %s is not compatible with SDK (supports %s)",
			serverVersion, APIVersionRange)
	}

	return result
}

// MustBeCompatible panics if the server version is not compatible.
//
// Use this in initialization code where incompatibility should be fatal:
//
//	func main() {
//	    client := stromboli.NewClient(url)
//	    health, err := client.Health(ctx)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    stromboli.MustBeCompatible(health.Version)
//	    // Continue with compatible server...
//	}
func MustBeCompatible(serverVersion string) {
	result := CheckCompatibility(serverVersion)
	if result.Status != Compatible {
		panic(fmt.Sprintf("stromboli: %s", result.Message))
	}
}
