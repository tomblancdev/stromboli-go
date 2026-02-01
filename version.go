package stromboli

// Version is the current SDK version.
//
// This version follows semantic versioning (https://semver.org/).
// The version is incremented according to the following rules:
//   - MAJOR: Breaking changes to the public API
//   - MINOR: New features, backwards compatible
//   - PATCH: Bug fixes, backwards compatible
const Version = "0.1.0"

// APIVersion is the target Stromboli API version this SDK was built for.
//
// The SDK is tested against this API version and may not work correctly
// with significantly different API versions. Use [Client.Health] to check
// the actual server version at runtime.
const APIVersion = "0.3.0-alpha"
