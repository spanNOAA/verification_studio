package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"verification_studio/internal/hpc"
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
  verification_studio hpc-run <args>   Run built-in hardened NOAA HPC read-only tool
  verification_studio hpc-auth <clstr> Authenticate (PIN + YubiKey) for target cluster

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
	// 1. Multi-call invocation when binary is invoked directly as hpc-run
	baseName := filepath.Base(args[0])
	if baseName == "hpc-run" {
		return runHPCCLI(args[1:])
	}

	if len(args) <= 1 {
		return false
	}

	firstArg := args[1]
	// Ignore macOS Finder process serial number flag (e.g. -psn_0_12345)
	if strings.HasPrefix(firstArg, "-psn") {
		return false
	}

	// 2. Subcommands for NOAA HPC read-only operations
	if firstArg == "hpc-auth" {
		return runHPCAuth(args[2:])
	}
	if firstArg == "hpc-worker" {
		return runHPCWorker(args[2:])
	}
	if firstArg == "hpc-run" {
		return runHPCCLI(args[2:])
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

func printHPCUsage() {
	fmt.Print(`==================================================================
 hpc-run: Hardened Read-Only Multi-Cluster SSH Access Tool (V1)
==================================================================
Protocol: RO-v1 (Native Pure Go Edition)

Supported Clusters:
`)
	for c, info := range hpc.Clusters {
		online, status := hpc.CheckStatus(c)
		mark := "○"
		if online {
			mark = "●"
		}
		fmt.Printf("  %s %-10s - %s [%s]\n", mark, c, info.Description, status)
	}

	fmt.Print(`
Permitted Structured Read-Only Operations (V1):
  verification_studio hpc-run <cluster> list <remote_dir>
      List remote directory entries (non-recursive)
  verification_studio hpc-run <cluster> stat <remote_path>
      Query remote file size and metadata
  verification_studio hpc-run <cluster> download <remote_path> <local_path>
      Download single remote file safely to local path (Max 50 MB)

Connection Management:
  verification_studio hpc-run --list                 List all supported clusters and live status
  verification_studio hpc-run --status [cluster|all] Check daemon status
  verification_studio hpc-run --start <cluster>      Authenticate (PIN + YubiKey) and start daemon
  verification_studio hpc-run --stop [cluster|all]   Stop background session daemon

Security Boundary:
  Arbitrary remote command execution, upload, delete, file modification,
  and job submission are permanently disabled by the read-only daemon.
`)
}

func runHPCAuth(args []string) bool {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: verification_studio hpc-auth <cluster> [--user <username>]")
		os.Exit(1)
	}
	cluster := args[0]
	candidateUser := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--user" && i+1 < len(args) {
			candidateUser = args[i+1]
			i++
		}
	}
	if err := hpc.StartInteractiveAuth(cluster, candidateUser); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
	return true
}

func runHPCWorker(args []string) bool {
	if len(args) == 0 {
		os.Exit(1)
	}
	cluster := args[0]
	targetUser := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--user" && i+1 < len(args) {
			targetUser = args[i+1]
			i++
		}
	}
	d, err := hpc.NewDaemon(cluster, targetUser)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	if err := d.RunServer(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
	return true
}

func runHPCCLI(args []string) bool {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "--list" {
		printHPCUsage()
		os.Exit(0)
	}

	cmd := args[0]
	switch cmd {
	case "--status":
		rawTarget := "all"
		if len(args) > 1 {
			rawTarget = strings.ToLower(args[1])
		}
		if rawTarget == "all" {
			for c := range hpc.Clusters {
				_, status := hpc.CheckStatus(c)
				fmt.Printf("%-10s: %s\n", c, status)
			}
		} else {
			target := hpc.NormalizeCluster(rawTarget, "")
			if target == "" {
				fmt.Fprintf(os.Stderr, "Unknown cluster: %s\n", rawTarget)
				os.Exit(1)
			}
			_, status := hpc.CheckStatus(target)
			fmt.Printf("%s: %s\n", target, status)
		}
		os.Exit(0)

	case "--start":
		if len(args) < 2 {
			fmt.Println("Please specify a cluster to start (e.g. verification_studio hpc-run --start gaea-c6)")
			os.Exit(1)
		}
		target := hpc.NormalizeCluster(args[1], "")
		if target == "" {
			fmt.Fprintf(os.Stderr, "Unknown cluster: %s\n", args[1])
			os.Exit(1)
		}
		if err := hpc.StartInteractiveAuth(target, ""); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)

	case "--stop":
		rawTarget := "all"
		if len(args) > 1 {
			rawTarget = strings.ToLower(args[1])
		}
		if rawTarget == "all" {
			hpc.DisconnectAll()
			fmt.Println("Stopped all background read-only daemons.")
		} else {
			target := hpc.NormalizeCluster(rawTarget, "")
			if target == "" {
				fmt.Fprintf(os.Stderr, "Unknown cluster: %s\n", rawTarget)
				os.Exit(1)
			}
			_ = hpc.StopDaemon(target)
			fmt.Printf("[%s] Stopped read-only daemon.\n", strings.ToUpper(target))
		}
		os.Exit(0)
	}

	cluster := hpc.NormalizeCluster(cmd, "")
	if cluster == "" {
		fmt.Fprintf(os.Stderr, "Error: Unknown cluster or option %q.\n", cmd)
		printHPCUsage()
		os.Exit(1)
	}

	subArgs := args[1:]
	if len(subArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: Missing read-only operation for cluster %q.\nAllowed operations: list, stat, download\n", cluster)
		os.Exit(1)
	}

	op := strings.ToLower(subArgs[0])
	switch op {
	case "list":
		if len(subArgs) < 2 {
			fmt.Fprintln(os.Stderr, "Error: 'list' requires a remote path argument.")
			os.Exit(1)
		}
		code, out, err := hpc.ClientSend(cluster, hpc.OpList, subArgs[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if code == 0 {
			fmt.Println(out)
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", out)
			os.Exit(code)
		}

	case "stat":
		if len(subArgs) < 2 {
			fmt.Fprintln(os.Stderr, "Error: 'stat' requires a remote path argument.")
			os.Exit(1)
		}
		code, out, err := hpc.ClientSend(cluster, hpc.OpStat, subArgs[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if code == 0 {
			fmt.Println(out)
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", out)
			os.Exit(code)
		}

	case "download":
		if len(subArgs) < 3 {
			fmt.Fprintln(os.Stderr, "Error: 'download' requires <remote_path> and <local_path> arguments.")
			os.Exit(1)
		}
		remotePath := subArgs[1]
		localPath := subArgs[2]
		code, out, err := hpc.ClientSend(cluster, hpc.OpDownload, remotePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
			os.Exit(1)
		}
		if code != 0 {
			fmt.Fprintf(os.Stderr, "Download failed: %s\n", out)
			os.Exit(code)
		}
		rawBytes, bErr := base64.StdEncoding.DecodeString(strings.TrimSpace(out))
		if bErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to decode remote payload: %v\n", bErr)
			os.Exit(1)
		}
		n, wErr := hpc.SafeAtomicWrite(rawBytes, localPath)
		if wErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to save local file: %v\n", wErr)
			os.Exit(1)
		}
		fmt.Printf("Successfully downloaded %s to %s (%d bytes).\n", remotePath, localPath, n)
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "\n[SECURITY BLOCKED] Arbitrary shell command execution is permanently disabled.\n")
		fmt.Fprintf(os.Stderr, "Attempted operation: %s\n", op)
		fmt.Fprintf(os.Stderr, "Permitted read-only operations:\n")
		fmt.Fprintf(os.Stderr, "  hpc-run %s list <remote_dir>\n", cluster)
		fmt.Fprintf(os.Stderr, "  hpc-run %s stat <remote_path>\n", cluster)
		fmt.Fprintf(os.Stderr, "  hpc-run %s download <remote_path> <local_path>\n\n", cluster)
		fmt.Fprintf(os.Stderr, "Error: READ_ONLY_OPERATION_DENIED\n")
		os.Exit(13)
	}
	return true
}
