package pipeline

import (
	"encoding/json"
	"testing"
)

func TestExtractShortName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"RRFSv2_conus_3km_ret_generictquv", "generictquv"},
		{"RRFSv2_conus_3km_ret_halfTerr", "halfTerr"},
		{"RRFSv2_conus_3km_ret_RRFSv2X_retv3", "retv3"},
		{"RRFSv2_conus_3km_ret_v2x_retv4", "retv4"},
		{"simple_model", "model"},
	}

	for _, tc := range cases {
		actual := ExtractShortName(tc.input)
		if actual != tc.expected {
			t.Errorf("ExtractShortName(%q) = %q; want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestParseModelEntry(t *testing.T) {
	m1 := ParseModelEntry("RRFSv2_conus_3km_ret_generictquv - gaeac6:/gpfs/f6/wrfruc/scratch/test")
	if m1.ID != "RRFSv2_conus_3km_ret_generictquv" {
		t.Errorf("Unexpected ID: %s", m1.ID)
	}
	if m1.Name != "generictquv" {
		t.Errorf("Unexpected Name: %s", m1.Name)
	}
	if m1.Cluster != "gaea-c6" {
		t.Errorf("Unexpected Cluster: %s", m1.Cluster)
	}
	if m1.Path != "/gpfs/f6/wrfruc/scratch/test" {
		t.Errorf("Unexpected Path: %s", m1.Path)
	}

	m2 := ParseModelEntry("customName : ID_001 - hera:/scratch1/RDARCH/dir")
	if m2.Name != "customName" {
		t.Errorf("Expected customName, got %s", m2.Name)
	}
	if m2.Cluster != "hera" {
		t.Errorf("Expected hera, got %s", m2.Cluster)
	}
}

func TestParseRadarCases(t *testing.T) {
	raw := json.RawMessage(`[
		{"date": "2024-05-08", "cycle": "00", "fhrs": [1, 3]},
		["2", "20240509", "12", 6, "6-h fcst from 12Z May 9"]
	]`)

	cases := ParseRadarCases(raw)
	if len(cases) != 3 {
		t.Fatalf("Expected 3 cases, got %d", len(cases))
	}

	if cases[0].Date != "20240508" || cases[0].Cycle != "00" || cases[0].Fhr != 1 {
		t.Errorf("Unexpected case 0: %+v", cases[0])
	}
	if cases[1].Date != "20240508" || cases[1].Cycle != "00" || cases[1].Fhr != 3 {
		t.Errorf("Unexpected case 1: %+v", cases[1])
	}
	if cases[2].Date != "20240509" || cases[2].Cycle != "12" || cases[2].Fhr != 6 {
		t.Errorf("Unexpected case 2: %+v", cases[2])
	}
}
