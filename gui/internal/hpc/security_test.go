package hpc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeRemotePath(t *testing.T) {
	validPaths := []string{
		"/gpfs/f6/scratch1/user/cref.png",
		"/gpfs/f5/data/my file [with spaces].png",
		"/scratch2/RDARCH/test_run/cref_20240506_12z_f01.png",
	}

	for _, p := range validPaths {
		sanitized, err := SanitizeRemotePath(p)
		if err != nil {
			t.Errorf("SanitizeRemotePath(%q) returned unexpected error: %v", p, err)
		}
		if sanitized != p {
			t.Errorf("SanitizeRemotePath(%q) = %q, expected %q", p, sanitized, p)
		}
	}

	invalidPaths := []string{
		"",
		"   ",
		"/path/with\nnewline",
		"/path/with\rescape",
		"/path/with\x1b[31mcolor",
		"/path/with\x7fDEL",
		strings.Repeat("a", 4097),
	}

	for _, p := range invalidPaths {
		_, err := SanitizeRemotePath(p)
		if err == nil {
			t.Errorf("SanitizeRemotePath(%q) should have failed but succeeded", p)
		}
	}
}

func TestQuotePosixPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "''"},
		{"/path/to/file.png", "'/path/to/file.png'"},
		{"/path/with 'single quotes'/file.png", `'/path/with '\''single quotes'\''/file.png'`},
	}

	for _, tc := range tests {
		actual := QuotePosixPath(tc.input)
		if actual != tc.expected {
			t.Errorf("QuotePosixPath(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestSafeAtomicWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hpc_security_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "sub", "test.png")
	data := []byte("PNG_TEST_MOCK_DATA")

	n, err := SafeAtomicWrite(data, testFile)
	if err != nil {
		t.Fatalf("SafeAtomicWrite failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected %d bytes written, got %d", len(data), n)
	}

	readBack, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read back written file: %v", err)
	}
	if string(readBack) != string(data) {
		t.Errorf("Read back data mismatch: got %q, expected %q", string(readBack), string(data))
	}
}
