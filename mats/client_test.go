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

func TestResolveLeadHours(t *testing.T) {
	// 1. Nil configured leads should fall back to defaults
	resNil := ResolveLeadHours(nil, DefaultSurfaceLeads)
	if len(resNil) != len(DefaultSurfaceLeads) {
		t.Errorf("Expected fallback to defaults when nil, got %v", resNil)
	}

	// 2. Explicitly empty leads slice should remain empty (skipped component)
	resEmpty := ResolveLeadHours([]string{}, DefaultSurfaceLeads)
	if len(resEmpty) != 0 {
		t.Errorf("Expected empty slice when configured as empty, got %v", resEmpty)
	}

	// 3. Configured leads should be sorted and deduplicated
	resCustom := ResolveLeadHours([]string{"24h", "3", "1h", "3h"}, DefaultSurfaceLeads)
	expected := []string{"1", "3", "24"}
	if len(resCustom) != len(expected) {
		t.Fatalf("Expected %v, got %v", expected, resCustom)
	}
	for i := range expected {
		if resCustom[i] != expected[i] {
			t.Errorf("Index %d: expected %s, got %s", i, expected[i], resCustom[i])
		}
	}
}

func TestBuildConfigsEmptyLeads(t *testing.T) {
	sfcConfigs := BuildSurfaceTSConfigs([]string{})
	if len(sfcConfigs) != 0 {
		t.Errorf("Expected 0 configs for empty surface leads, got %d", len(sfcConfigs))
	}

	uaConfigs := BuildUAProfileConfigs([]string{})
	if len(uaConfigs) != 0 {
		t.Errorf("Expected 0 configs for empty UA leads, got %d", len(uaConfigs))
	}
}
