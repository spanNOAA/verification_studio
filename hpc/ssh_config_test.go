package hpc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferUserFromModelPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/gpfs/f6/sijie.pan/rrfs_v2/output", "sijie.pan"},
		{"/gpfs/f6/wrfruc/scratch/username/retros/exp1", "username"},
		{"/scratch1/RDARCH/eda.hab/model_runs/test", "eda.hab"},
		{"/scratch2/BMC/gsd-hpcs/john.doe/run", "BMC"}, // scratch2 match
		{"/work/noaa/r2d/forecast", "r2d"},
		{"/var/tmp/non_hpc/path", ""},
	}

	for _, tc := range tests {
		actual := InferUserFromModelPath(tc.path)
		if actual != tc.expected {
			t.Errorf("InferUserFromModelPath(%q) = %q, expected %q", tc.path, actual, tc.expected)
		}
	}
}

func TestResolveSSHHostFallback(t *testing.T) {
	// Without SSH config alias, should return Canonical FQDN
	host := ResolveSSHHost("gaea-c6")
	expected := "gaea-c6.fairmont.rdhpcs.noaa.gov"
	if host != expected && host != "Gaeac6" {
		t.Errorf("Expected host fallback %q or Gaeac6, got %q", expected, host)
	}

	heraHost := ResolveSSHHost("hera")
	expectedHera := "hera-mfa.fairmont.rdhpcs.noaa.gov"
	if heraHost != expectedHera && heraHost != "Hera" {
		t.Errorf("Expected host fallback %q or Hera, got %q", expectedHera, heraHost)
	}
}

func TestBuildSSHArgs(t *testing.T) {
	args := BuildSSHArgs("gaea-c6.fairmont.rdhpcs.noaa.gov", "sijie.pan")
	expectedHost := "gaea-c6.fairmont.rdhpcs.noaa.gov"

	if args[len(args)-1] != expectedHost {
		t.Errorf("Expected final argument to be %q, got %q", expectedHost, args[len(args)-1])
	}

	hasUser := false
	for i, a := range args {
		if a == "-l" && i+1 < len(args) && args[i+1] == "sijie.pan" {
			hasUser = true
			break
		}
	}
	if !hasUser {
		t.Errorf("Expected -l sijie.pan in args, got %v", args)
	}
}

func TestSaveConfirmedUser(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "hpc_home_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)

	t.Setenv("HOME", tempHome)

	err = SaveConfirmedUser("gaea-c6", "test_user_noaa")
	if err != nil {
		t.Fatalf("SaveConfirmedUser failed: %v", err)
	}

	clusterFile := filepath.Join(tempHome, ".hpc_user_gaea-c6")
	data, err := os.ReadFile(clusterFile)
	if err != nil {
		t.Fatalf("Failed to read back cluster user file: %v", err)
	}
	if string(data) != "test_user_noaa\n" {
		t.Errorf("Expected file content 'test_user_noaa\\n', got %q", string(data))
	}
}
