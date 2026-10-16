package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"verification_studio/internal/hpc"
	"verification_studio/internal/mats"
)

func TestBuildReports(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "report_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	matsDir := filepath.Join(tempDir, "mats")
	crefDir := filepath.Join(tempDir, "cref")
	_ = os.MkdirAll(matsDir, 0o755)
	_ = os.MkdirAll(crefDir, 0o755)

	// Create a dummy image
	dummyImg := filepath.Join(matsDir, "surface_dieoff_t2m.png")
	if err := os.WriteFile(dummyImg, []byte(strings.Repeat("DUMMY_PNG_DATA", 100)), 0o644); err != nil {
		t.Fatalf("Failed to write dummy image: %v", err)
	}

	cfg := Config{
		Title: "Test Verification Suite",
		Dates: DatesConfig{
			Surface:  "05/06/2024 - 05/13/2024",
			UpperAir: "05/06/2024 - 05/13/2024",
		},
		Models: []mats.Model{
			{ID: "m1", Name: "ModelA", Color: "#2563eb"},
			{ID: "m2", Name: "ModelB", Color: "#dc2626"},
		},
		Regions: []string{"all", "east"},
		CrefDir: crefDir,
		RadarCases: []hpc.RadarCase{
			{Index: "1", Date: "20240508", Cycle: "00", Fhr: 1, Title: "1-h fcst from 00Z May 8"},
		},
	}

	pptxPath := filepath.Join(tempDir, "test_output.pptx")
	slidesData, err := BuildPresentation(cfg, matsDir, pptxPath)
	if err != nil {
		t.Fatalf("BuildPresentation failed: %v", err)
	}

	if len(slidesData) == 0 {
		t.Errorf("Expected slidesData to be non-empty")
	}

	info, err := os.Stat(pptxPath)
	if err != nil || info.Size() == 0 {
		t.Errorf("Generated PPTX does not exist or is empty")
	}
	t.Logf("Generated PPTX: %s (%d bytes, %d slides)", pptxPath, info.Size(), len(slidesData))

	htmlPath := filepath.Join(tempDir, "test_output.html")
	if err := BuildHTMLReport(cfg, matsDir, htmlPath, slidesData); err != nil {
		t.Fatalf("BuildHTMLReport failed: %v", err)
	}

	hInfo, err := os.Stat(htmlPath)
	if err != nil || hInfo.Size() == 0 {
		t.Errorf("Generated HTML does not exist or is empty")
	}
	t.Logf("Generated HTML: %s (%d bytes)", htmlPath, hInfo.Size())

	// Verify HTML content has region tabs and no redundant Show All
	htmlBytes, _ := os.ReadFile(htmlPath)
	htmlContent := string(htmlBytes)
	if !strings.Contains(htmlContent, "🌐 Full Domain") {
		t.Errorf("HTML does not contain Full Domain tab")
	}
	if !strings.Contains(htmlContent, "🗺️ Eastern Region") {
		t.Errorf("HTML does not contain Eastern Region tab")
	}
	if strings.Contains(htmlContent, "Show All") || strings.Contains(htmlContent, "Show all") {
		t.Errorf("HTML should not contain redundant Show All button")
	}
}
