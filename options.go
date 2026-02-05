package stromboli

import (
	"log"
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
//	    stromboli.WithHTTPClient(customHTTPClient),
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
// Deprecated: Retry logic is not implemented. This option logs a warning
// and does nothing. Implement retry logic in your application or use a
// library like hashicorp/go-retryablehttp. This will be removed in v1.0.
//
// Default: 0 (no retries).
func WithRetries(n int) Option {
	return func(_ *Client) {
		if n > 0 {
			log.Printf("stromboli: WARNING: WithRetries(%d) is deprecated and has no effect", n)
		}
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
		if userAgent != "" {
			c.userAgent = userAgent
		}
		// If empty, keep the default "stromboli-go/{version}"
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

// RequestHook is called before each HTTP request is sent.
// Use this for logging, metrics, or modifying requests.
type RequestHook func(req *http.Request)

// ResponseHook is called after each HTTP response is received.
// Use this for logging, metrics, or inspecting responses.
type ResponseHook func(resp *http.Response)

// WithRequestHook sets a hook that is called before each HTTP request.
//
// Use this for observability (logging, metrics) or to modify requests
// before they are sent.
//
// Example:
//
//	client, err := stromboli.NewClient(url,
//	    stromboli.WithRequestHook(func(req *http.Request) {
//	        log.Printf("Request: %s %s", req.Method, req.URL)
//	    }),
//	)
func WithRequestHook(hook RequestHook) Option {
	return func(c *Client) {
		c.requestHook = hook
	}
}

// WithResponseHook sets a hook that is called after each HTTP response.
//
// Use this for observability (logging, metrics) or to inspect responses.
// Note: The response body may have already been read by the client.
//
// Example:
//
//	client, err := stromboli.NewClient(url,
//	    stromboli.WithResponseHook(func(resp *http.Response) {
//	        log.Printf("Response: %d %s", resp.StatusCode, resp.Status)
//	    }),
//	)
func WithResponseHook(hook ResponseHook) Option {
	return func(c *Client) {
		c.responseHook = hook
	}
}
