package stromboli

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// Logger is the interface used for SDK logging.
// Implement this interface to customize log output.
type Logger interface {
	Printf(format string, v ...interface{})
}

// defaultLogger wraps the standard log package.
type defaultLogger struct{}

func (defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// sdkLoggerMu protects sdkLogger for concurrent access.
var sdkLoggerMu sync.RWMutex

// sdkLogger is the logger used by the SDK for warnings and debug output.
// Can be replaced via SetLogger. Access must be protected by sdkLoggerMu.
var sdkLogger Logger = defaultLogger{}

// SetLogger sets the logger used by the SDK for warnings and debug output.
// Pass nil to restore the default logger (standard log package).
// This function is safe for concurrent use.
//
// Example:
//
//	// Use a custom logger
//	stromboli.SetLogger(myLogger)
//
//	// Restore default
//	stromboli.SetLogger(nil)
func SetLogger(l Logger) {
	sdkLoggerMu.Lock()
	defer sdkLoggerMu.Unlock()
	if l == nil {
		sdkLogger = defaultLogger{}
	} else {
		sdkLogger = l
	}
}

// getLogger returns the current logger (thread-safe).
func getLogger() Logger {
	sdkLoggerMu.RLock()
	defer sdkLoggerMu.RUnlock()
	return sdkLogger
}

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

// WithStreamTimeout sets the default timeout for streaming requests.
//
// Unlike regular requests, streams are long-running connections where data
// arrives incrementally. This timeout applies only if no context deadline
// is set when calling [Client.Stream].
//
// If not set, streaming requests have no timeout by default. This can be
// dangerous as a stalled server may cause the client to hang indefinitely.
// It's recommended to either set this option or use context.WithTimeout.
//
// A timeout of zero or negative disables the stream timeout (not recommended).
//
// Example:
//
//	client, err := stromboli.NewClient(url,
//	    stromboli.WithStreamTimeout(5*time.Minute),
//	)
//
//	// Now Stream will automatically timeout after 5 minutes if no context deadline is set
//	stream, err := client.Stream(ctx, req)
func WithStreamTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.streamTimeout = d
		}
	}
}

// WithRetries sets the maximum number of retry attempts for failed requests.
//
// Deprecated: Retry logic is not implemented. This option logs a warning
// and does nothing. Consider using:
//   - github.com/hashicorp/go-retryablehttp for automatic retries
//   - github.com/cenkalti/backoff for custom retry logic
//   - github.com/avast/retry-go for simple retry patterns
//
// This option will be removed in v1.0.
//
// Note: The deprecation warning is logged when [NewClient] is called.
// If you use [SetLogger] to configure a custom logger, call it before
// creating clients to see this warning in your logger.
//
// Default: 0 (no retries).
func WithRetries(n int) Option {
	return func(_ *Client) {
		if n > 0 {
			getLogger().Printf("stromboli: WARNING: WithRetries(%d) is deprecated and has no effect", n)
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
// Passing nil is a no-op and the default client is retained.
// This is typically a programmer error; consider checking for nil before calling.
//
// Default: A new [http.Client] with cloned [http.DefaultTransport].
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
		// nil is silently ignored - default client is retained
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
// Pass an empty string to clear any previously set token.
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
		// Validate token to prevent HTTP header injection via CR/LF characters.
		// Empty string is valid (clears token), but non-empty tokens must be safe.
		if token != "" && !isValidToken(token) {
			getLogger().Printf("stromboli: WARNING: WithToken called with invalid token (contains control characters), ignoring")
			return
		}
		c.token = token
	}
}

// RequestHook is called before each HTTP request is sent.
// Use this for logging, metrics, or modifying requests.
type RequestHook func(req *http.Request)

// ResponseHook is called after each HTTP response is received.
//
// WARNING: For most API methods (Run, Health, etc.), the response body will
// be consumed by the generated client before your hook runs. The hook is
// primarily useful for inspecting headers and status codes, not body content.
// For the Stream method, the body is available as it hasn't been consumed yet.
//
// Use this for logging, metrics, or inspecting response metadata.
type ResponseHook func(resp *http.Response)

// WithRequestHook sets a hook that is called before each HTTP request.
//
// Use this for observability (logging, metrics) or to modify requests
// before they are sent. Pass nil to clear a previously set hook.
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
		c.requestHook = hook // nil is valid (clears hook)
	}
}

// WithResponseHook sets a hook that is called after each HTTP response.
//
// Use this for observability (logging, metrics) or to inspect response headers
// and status codes. See [ResponseHook] for important caveats about body availability.
// Pass nil to clear a previously set hook.
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
		c.responseHook = hook // nil is valid (clears hook)
	}
}
