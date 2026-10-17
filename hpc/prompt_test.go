package hpc

import (
	"regexp"
	"testing"
)

func TestPromptDetection(t *testing.T) {
	c6Regex := Clusters["gaea-c6"].PromptRegex
	c5Regex := Clusters["gaea-c5"].PromptRegex
	c7Regex := Clusters["gaea-c7"].PromptRegex
	heraRegex := Clusters["hera"].PromptRegex
	ursaRegex := Clusters["ursa"].PromptRegex
	orionRegex := Clusters["orion"].PromptRegex
	herculesRegex := Clusters["hercules"].PromptRegex

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Gaea-C6 c6login bash prompt",
			input:    "[Sijie.Pan@c6login1 ~]$ ",
			expected: true,
		},
		{
			name:     "Gaea-C6 c6login01 prompt with path",
			input:    "[Sijie.Pan@c6login01 /gpfs/f6/wrfruc]$ ",
			expected: true,
		},
		{
			name:     "Gaea-C6 tcsh prompt",
			input:    "Sijie.Pan@c6login1:~> ",
			expected: true,
		},
		{
			name:     "Gaea-C6 gaea-c6 alias prompt",
			input:    "[Sijie.Pan@gaea-c6 ~]$ ",
			expected: true,
		},
		{
			name:     "Gaea-C5 gaea51 prompt",
			input:    "[Sijie.Pan@gaea51 ~]$ ",
			expected: true,
		},
		{
			name:     "Gaea-C7 c7login prompt",
			input:    "[Sijie.Pan@c7login1 ~]$ ",
			expected: true,
		},
		{
			name:     "Hera hfe prompt",
			input:    "[Sijie.Pan@hfe01 ~]$ ",
			expected: true,
		},
		{
			name:     "Ursa ufe prompt",
			input:    "[Sijie.Pan@ufe01 ~]$ ",
			expected: true,
		},
		{
			name:     "Orion login prompt",
			input:    "[span@Orion-login-1 ~]$ ",
			expected: true,
		},
		{
			name:     "Hercules login prompt",
			input:    "[span@hercules-login-1 ~]$ ",
			expected: true,
		},
		{
			name:     "ANSI colored bash prompt",
			input:    "\x1b[0;32m[Sijie.Pan@c6login1\x1b[0m \x1b[0;34m~\x1b[0m]$ ",
			expected: true,
		},
		{
			name:     "Bracketed paste mode trailing escape",
			input:    "[Sijie.Pan@c6login1 ~]$ \x1b[?2004h",
			expected: true,
		},
		{
			name: "Full login MOTD banner with prompt at the end",
			input: `****************************************************************************
                          NOTICE TO USERS
****************************************************************************
Last login: Sat Oct 17 17:06:17 2026 from gaea-mfa.fairmont.rdhpcs.noaa.gov

[Sijie.Pan@c6login1 ~]$ `,
			expected: true,
		},
		{
			name: "Full login MOTD banner WITHOUT prompt yet",
			input: `****************************************************************************
                          NOTICE TO USERS
****************************************************************************
Last login: Sat Oct 17 17:06:17 2026 from gaea-mfa.fairmont.rdhpcs.noaa.gov
`,
			expected: false,
		},
		{
			name:     "PIN prompt",
			input:    "Enter your PIN + OTP: ",
			expected: false,
		},
		{
			name:     "Password prompt",
			input:    "Password: ",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := []byte(tc.input)
			cleaned := ansiCleaner.ReplaceAll(buf, []byte(""))

			// Test against c6 regex and generic
			matched := IsPromptDetected(buf, cleaned, c6Regex)
			if matched != tc.expected {
				t.Errorf("Expected IsPromptDetected = %v for %q, got %v", tc.expected, tc.input, matched)
			}
		})
	}

	// Verify all cluster regexes independently
	clusterChecks := []struct {
		name  string
		regex *regexp.Regexp
		input string
	}{
		{"Gaea-C5", c5Regex, "[user@gaea51 ~]$ "},
		{"Gaea-C6", c6Regex, "[user@c6login2 ~]$ "},
		{"Gaea-C7", c7Regex, "[user@c7login1 ~]$ "},
		{"Hera", heraRegex, "[user@hfe04 ~]$ "},
		{"Ursa", ursaRegex, "[user@ufe02 ~]$ "},
		{"Orion", orionRegex, "[span@Orion-login-2 ~]$ "},
		{"Hercules", herculesRegex, "[span@hercules-login-1 ~]$ "},
	}

	for _, cc := range clusterChecks {
		t.Run(cc.name, func(t *testing.T) {
			buf := []byte(cc.input)
			cleaned := ansiCleaner.ReplaceAll(buf, []byte(""))
			if !IsPromptDetected(buf, cleaned, cc.regex) {
				t.Errorf("Failed to detect prompt for %s on %q", cc.name, cc.input)
			}
		})
	}
}
