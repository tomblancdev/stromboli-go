package stromboli

import (
	"net/http"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 0
)

// Client is the Stromboli API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	maxRetries int
}

// NewClient creates a new Stromboli client.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
		timeout:    defaultTimeout,
		maxRetries: defaultMaxRetries,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
