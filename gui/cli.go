package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"verification_studio/internal/pipeline"
)

func printUsage() {
	fmt.Print(`==================================================================
 NOAA GSL Verification Studio v1.0.0 (Pure Go Universal Edition)
==================================================================
A native, lightweight numerical weather prediction model verification
and presentation generator suite.

Usage:
  verification_studio                  Launch native macOS desktop GUI
  verification_studio [options]        Run verification pipeline in headless CLI mode
  verification_studio --config <file>  Execute pipeline from YAML or JSON configuration

CLI Flags:
  --config, -c <path>     Path to YAML or JSON configuration file
  --title <string>        Presentation title (default: "RRFSv2 Model Verification & Performance Evaluation")
  --output-dir, -o <path> Directory to save downloaded charts and generated reports
  --report-name <string>  Base name for .pptx and .html outputs (default: "mats_report")
  --models <text>         Models to verify (format: "alias:model_id - machine:path" or "model_id")
  --sfc-dates <range>     Surface date range (e.g. "05/06/2024 10:00 - 05/13/2024 00:00")
  --ua-dates <range>      Upper-air date range (e.g. "05/06/2024 00:00 - 05/13/2024 12:00")
  --regions <list>        Comma-separated evaluated regions (e.g. "all", "all,east,west")
  --skip-fetch            Skip downloading and build report directly from existing cached charts
  --force-fetch           Force fresh downloads from NOAA MATS even if cached
  --include-cref          Include synoptic radar reflectivity case comparisons
  --help, -h              Show this help message
  --version, -v           Show software version information

Examples:
  # 1. Run via configuration file:
  ./verification_studio --config config_example.yaml

  # 2. Run via command-line options:
  ./verification_studio \
    --models "retv4:RRFSv2_conus_3km_ret_v2x_retv4" \
    --sfc-dates "05/06/2024 10:00 - 05/13/2024 00:00" \
    --ua-dates "05/06/2024 00:00 - 05/13/2024 12:00" \
    --output-dir ~/Desktop/my_run \
    --report-name "My_Verification_Deck"
`)
}

type fileConfigModel struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	Cluster string `json:"cluster" yaml:"cluster"`
	Path    string `json:"path" yaml:"path"`
}

type fileConfigDates struct {
	Surface  string `json:"surface" yaml:"surface"`
	UpperAir string `json:"upperair" yaml:"upperair"`
}

type fileConfig struct {
	Title       string            `json:"title" yaml:"title"`
	OutputDir   string            `json:"output_dir" yaml:"output_dir"`
	ReportName  string            `json:"report_name" yaml:"report_name"`
	PptxName    string            `json:"output_pptx" yaml:"output_pptx"`
	ModelsText  string            `json:"models_text" yaml:"models_text"`
	Models      []fileConfigModel `json:"models" yaml:"models"`
	Dates       fileConfigDates   `json:"dates" yaml:"dates"`
	Regions     []string          `json:"regions" yaml:"regions"`
	SkipFetch   bool              `json:"skip_fetch" yaml:"skip_fetch"`
	ForceFetch  bool              `json:"force_fetch" yaml:"force_fetch"`
	IncludeCref bool              `json:"include_cref" yaml:"include_cref"`
	RadarCases  json.RawMessage   `json:"radar_cases" yaml:"radar_cases"`
}

func loadConfigFile(path string) (pipeline.Request, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pipeline.Request{}, fmt.Errorf("failed to read file: %w", err)
	}

	var cf fileConfig
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".json") {
		if err := json.Unmarshal(data, &cf); err != nil {
			return pipeline.Request{}, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &cf); err != nil {
			return pipeline.Request{}, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	}

	var modelEntries []string
	if cf.ModelsText != "" {
		modelEntries = append(modelEntries, cf.ModelsText)
	}
	for _, m := range cf.Models {
		entry := m.ID
		if m.Name != "" && m.Name != m.ID {
			entry = m.Name + ":" + m.ID
		}
		if m.Cluster != "" && m.Path != "" {
			entry += fmt.Sprintf(" - %s:%s", m.Cluster, m.Path)
		} else if m.Path != "" {
			entry += fmt.Sprintf(" - %s", m.Path)
		}
		modelEntries = append(modelEntries, entry)
	}

	baseName := cf.ReportName
	if baseName == "" && cf.PptxName != "" {
		fn := filepath.Base(cf.PptxName)
		baseName = strings.TrimSuffix(fn, filepath.Ext(fn))
	}
	if baseName == "" {
		baseName = "mats_report"
	}

	title := cf.Title
	if title == "" {
		title = "RRFSv2 Model Verification & Performance Evaluation"
	}

	regions := cf.Regions
	if len(regions) == 0 {
		regions = []string{"all"}
	}

	return pipeline.Request{
		Title:       title,
		OutputDir:   cf.OutputDir,
		ReportName:  baseName,
		ModelsText:  strings.Join(modelEntries, "\n"),
		SfcDates:    cf.Dates.Surface,
		UaDates:     cf.Dates.UpperAir,
		Regions:     regions,
		SkipFetch:   cf.SkipFetch,
		ForceFetch:  cf.ForceFetch,
		IncludeCref: cf.IncludeCref,
		RadarCases:  cf.RadarCases,
	}, nil
}

func runCLI(args []string) bool {
	if len(args) <= 1 {
		return false
	}

	firstArg := args[1]
	// Ignore macOS Finder process serial number flag (e.g. -psn_0_12345)
	if strings.HasPrefix(firstArg, "-psn") {
		return false
	}

	if firstArg == "--help" || firstArg == "-h" || firstArg == "help" {
		printUsage()
		os.Exit(0)
	}

	if firstArg == "--version" || firstArg == "-v" || firstArg == "version" {
		fmt.Println("NOAA GSL Verification Studio v1.0.0 (Pure Go Edition)")
		os.Exit(0)
	}

	fs := flag.NewFlagSet("verification_studio", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to YAML or JSON configuration file")
	fs.StringVar(configPath, "c", "", "Path to YAML or JSON configuration file (shorthand)")
	title := fs.String("title", "RRFSv2 Model Verification & Performance Evaluation", "Presentation title")
	outDir := fs.String("output-dir", "", "Directory to save charts and reports")
	fs.StringVar(outDir, "o", "", "Directory to save charts and reports (shorthand)")
	reportName := fs.String("report-name", "mats_report", "Base name for generated reports")
	modelsText := fs.String("models", "", "Models list text (comma-separated or one per line)")
	sfcDates := fs.String("sfc-dates", "05/06/2024 10:00 - 05/13/2024 00:00", "Surface date range")
	uaDates := fs.String("ua-dates", "05/06/2024 00:00 - 05/13/2024 12:00", "Upper-air date range")
	regionsStr := fs.String("regions", "all", "Comma-separated regions (all,east,west)")
	skipFetch := fs.Bool("skip-fetch", false, "Skip downloading charts; use local cache")
	forceFetch := fs.Bool("force-fetch", false, "Force fresh downloads from NOAA MATS")
	includeCref := fs.Bool("include-cref", false, "Include synoptic radar reflectivity case comparisons")
	isCLI := fs.Bool("cli", false, "Explicit headless CLI mode flag")

	if err := fs.Parse(args[1:]); err != nil {
		printUsage()
		os.Exit(1)
	}

	// If no CLI options were actually provided, fall back to GUI
	if *configPath == "" && !*isCLI && *outDir == "" && *modelsText == "" {
		return false
	}

	var req pipeline.Request
	if *configPath != "" {
		loadedReq, err := loadConfigFile(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to load config file: %v\n", err)
			os.Exit(1)
		}
		req = loadedReq
		// Command-line flags override config file if explicitly provided
		if *outDir != "" {
			req.OutputDir = *outDir
		}
		if *reportName != "mats_report" {
			req.ReportName = *reportName
		}
	} else {
		var regions []string
		for _, r := range strings.Split(*regionsStr, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				regions = append(regions, r)
			}
		}
		mText := strings.ReplaceAll(*modelsText, ",", "\n")
		req = pipeline.Request{
			Title:       *title,
			OutputDir:   *outDir,
			ReportName:  *reportName,
			ModelsText:  mText,
			SfcDates:    *sfcDates,
			UaDates:     *uaDates,
			Regions:     regions,
			SkipFetch:   *skipFetch,
			ForceFetch:  *forceFetch,
			IncludeCref: *includeCref,
		}
	}

	if req.OutputDir == "" {
		fmt.Fprintf(os.Stderr, "[ERROR] Output directory is required. Specify via --output-dir <path> or in config file.\n")
		os.Exit(1)
	}

	// Expand ~ in path
	if strings.HasPrefix(req.OutputDir, "~/") {
		home, _ := os.UserHomeDir()
		req.OutputDir = filepath.Join(home, req.OutputDir[2:])
	}

	fmt.Println("==================================================================")
	fmt.Println(" NOAA GSL Verification Studio — Headless CLI Execution")
	fmt.Printf(" Title      : %s\n", req.Title)
	fmt.Printf(" Output Dir : %s\n", req.OutputDir)
	fmt.Printf(" Report Name: %s\n", req.ReportName)
	fmt.Println("==================================================================")

	sendOutput := func(msg string) {
		fmt.Print(msg)
	}

	ctx := context.Background()
	if err := pipeline.Run(ctx, req, sendOutput); err != nil {
		fmt.Fprintf(os.Stderr, "\n[FATAL ERROR] Pipeline execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n[SUCCESS] Verification presentation deck and interactive HTML ready:\n")
	fmt.Printf("  • PPT : %s\n", filepath.Join(req.OutputDir, req.ReportName+".pptx"))
	fmt.Printf("  • HTML: %s\n", filepath.Join(req.OutputDir, req.ReportName+".html"))
	os.Exit(0)
	return true
}
