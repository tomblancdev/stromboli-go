package stromboli

import (
	"net/http"
	"time"
)

// Option configures a Client.
type Option func(*Client)

// WithTimeout sets the default request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = d
	}
}

// WithRetries sets the maximum number of retries for failed requests.
func WithRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}
