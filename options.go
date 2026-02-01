package stromboli

import (
	"net/http"
	"time"
)

// Option configures a [Client].
//
// Options are passed to [NewClient] to customize the client behavior.
// Multiple options can be combined:
//
//	client, err := stromboli.NewClient("http://localhost:8585",
//	    stromboli.WithTimeout(5*time.Minute),
//	    stromboli.WithRetries(3),
//	)
//
// Options are applied in order, so later options override earlier ones.
type Option func(*Client)

// WithTimeout sets the default timeout for all requests.
//
// The timeout applies to the entire request lifecycle, including
// connection establishment, request sending, and response reading.
// A timeout of zero means no timeout. Negative values are treated as zero.
//
// Default: 30 seconds.
//
// Example:
//
//	client, err := stromboli.NewClient(url,
//	    stromboli.WithTimeout(5*time.Minute), // Long timeout for slow operations
//	)
//
// For per-request timeouts, use context.WithTimeout instead:
//
//	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
//	defer cancel()
//	result, err := client.Health(ctx)
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d < 0 {
			d = 0
		}
		c.timeout = d
	}
}

// WithRetries sets the maximum number of retry attempts for failed requests.
//
// NOTE: Retry logic is planned but not yet implemented in v0.x.
// This option is reserved for future use. Implement retry logic in your
// application or use a library like hashicorp/go-retryablehttp.
//
// Negative values are treated as zero.
//
// Default: 0 (no retries).
//
// Example:
//
//	client, err := stromboli.NewClient(url,
//	    stromboli.WithRetries(3), // Retry up to 3 times
//	)
func WithRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// WithHTTPClient sets a custom HTTP client for making requests.
//
// Use this option to customize transport settings like:
//   - TLS configuration
//   - Proxy settings
//   - Connection pooling
//   - Custom transports (e.g., for testing)
//
// The provided client's Timeout field is ignored in favor of
// [WithTimeout]. Use [WithTimeout] to control request timeouts.
//
// Default: [http.DefaultClient].
//
// Example:
//
//	httpClient := &http.Client{
//	    Transport: &http.Transport{
//	        MaxIdleConns:        100,
//	        MaxIdleConnsPerHost: 10,
//	        IdleConnTimeout:     90 * time.Second,
//	    },
//	}
//	client := stromboli.NewClient(url,
//	    stromboli.WithHTTPClient(httpClient),
//	)
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
		// If nil, keep the default http.DefaultClient
	}
}

// WithUserAgent sets a custom User-Agent header for all requests.
//
// The User-Agent is sent with every request and can be used for
// server-side analytics or debugging.
//
// Default: "stromboli-go/{version}".
//
// Example:
//
//	client := stromboli.NewClient(url,
//	    stromboli.WithUserAgent("my-app/1.0.0"),
//	)
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// WithToken sets the Bearer token for authenticated requests.
//
// Use this option when you already have a valid access token and
// want to create an authenticated client from the start.
//
// Alternatively, use [Client.SetToken] to set the token after
// client creation, or [Client.GetToken] to obtain a new token.
//
// Example:
//
//	client := stromboli.NewClient(url,
//	    stromboli.WithToken("my-access-token"),
//	)
//
//	// Authenticated endpoints now work
//	validation, err := client.ValidateToken(ctx)
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}
