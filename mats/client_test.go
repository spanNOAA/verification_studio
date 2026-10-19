package mats

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		// 1. Business & Plotly rendering timeouts (must NOT be treated as rate limits)
		{
			name:     "Plotly canvas render timeout",
			err:      fmt.Errorf("timed out waiting for Plotly canvas to render"),
			expected: false,
		},
		{
			name:     "Generic element waiting timeout",
			err:      fmt.Errorf("timeout waiting for selector #plot-done"),
			expected: false,
		},

		// 2. Genuine rate limiting and gateway errors (MUST be treated as rate limits)
		{
			name:     "Explicit ErrRateLimited",
			err:      ErrRateLimited,
			expected: true,
		},
		{
			name:     "Wrapped ErrRateLimited",
			err:      fmt.Errorf("fetch error: %w", ErrRateLimited),
			expected: true,
		},
		{
			name:     "HTTP 429 message",
			err:      errors.New("HTTP 429 Too Many Requests"),
			expected: true,
		},
		{
			name:     "HTTP 503 Service Unavailable",
			err:      errors.New("503 Service Temporarily Unavailable"),
			expected: true,
		},
		{
			name:     "HTTP 504 Gateway Timeout",
			err:      errors.New("504 Gateway Time-out"),
			expected: true,
		},
		{
			name:     "Cloudflare challenge",
			err:      errors.New("cloudflare challenge detected"),
			expected: true,
		},
		{
			name:     "Chrome connection timeout",
			err:      errors.New("net::ERR_CONNECTION_TIMED_OUT"),
			expected: true,
		},
		{
			name:     "Chrome err timed out",
			err:      errors.New("net::ERR_TIMED_OUT"),
			expected: true,
		},
		{
			name:     "Connection reset by peer",
			err:      errors.New("read: connection reset by peer"),
			expected: true,
		},
		{
			name:     "Connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRateLimited(tt.err)
			if got != tt.expected {
				t.Errorf("isRateLimited(%v) = %v, expected %v", tt.err, got, tt.expected)
			}
		})
	}
}
