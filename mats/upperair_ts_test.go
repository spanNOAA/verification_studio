package mats

import (
	"testing"
)

func TestBuildUpperAirTSConfigs(t *testing.T) {
	// Test 1: Empty leads returns nil (skip)
	emptyConfigs := BuildUpperAirTSConfigs([]string{})
	if len(emptyConfigs) != 0 {
		t.Fatalf("expected 0 configs for empty leads, got %d", len(emptyConfigs))
	}

	// Test 2: 2 leads with 3 vars x 2 stats
	leads := []string{"3", "1"}
	configs := BuildUpperAirTSConfigs(leads)
	// 3 vars * 2 stats * 2 leads = 12 configs
	expectedCount := len(UpperAirTSVars) * len(UpperAirTSStats) * len(leads)
	if len(configs) != expectedCount {
		t.Fatalf("expected %d configs, got %d", expectedCount, len(configs))
	}

	// Verify that leads are sorted: "1" comes before "3"
	first := configs[0]
	if first.Fhr != "1" {
		t.Errorf("expected first config lead hour to be '1', got %s", first.Fhr)
	}
	if first.VarLabel != "Temperature" {
		t.Errorf("expected VarLabel 'Temperature', got %s", first.VarLabel)
	}
	if first.Stat != "RMSE" {
		t.Errorf("expected Stat 'RMSE', got %s", first.Stat)
	}
}
