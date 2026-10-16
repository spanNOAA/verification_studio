package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/webview/webview_go"
	"verification_studio/internal/pipeline"
)

//go:embed ui.html
var uiHTML string

type PipelineRequest struct {
	Title       string          `json:"title"`
	OutputDir   string          `json:"output_dir"`
	ModelsText  string          `json:"models_text"`
	SfcDates    string          `json:"sfc_dates"`
	UaDates     string          `json:"ua_dates"`
	ReportName  string          `json:"report_name"`
	PptxName    string          `json:"pptx_name"`
	SkipFetch   bool            `json:"skip_fetch"`
	ForceFetch  bool            `json:"force_fetch"`
	IncludeCref bool            `json:"include_cref"`
	RadarCases  json.RawMessage `json:"radar_cases"`
	Regions     []string        `json:"regions"`
}

var (
	allowedPathsLock sync.RWMutex
	allowedPathsMap  = make(map[string]bool)

	pipelineMutex      sync.Mutex
	pipelineCancel     context.CancelFunc
	isPipelineRunning  bool
	isPipelineStopping bool
)

func allowOpenPath(p string) {
	if p == "" {
		return
	}
	clean := filepath.Clean(p)
	if real, err := filepath.EvalSymlinks(clean); err == nil {
		clean = real
	}
	allowedPathsLock.Lock()
	defer allowedPathsLock.Unlock()
	allowedPathsMap[clean] = true
}

func validateDesktopDir(outDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return "", fmt.Errorf("unable to determine user home directory")
	}
	desktopDir := filepath.Join(home, "Desktop")
	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to access Desktop directory: %w", err)
	}

	realDesktop, err := filepath.EvalSymlinks(desktopDir)
	if err != nil {
		realDesktop = filepath.Clean(desktopDir)
	}

	cleanOut := filepath.Clean(outDir)
	if err := os.MkdirAll(cleanOut, 0o755); err != nil {
		return "", fmt.Errorf("failed to create output directory %s: %w", cleanOut, err)
	}

	realOut, err := filepath.EvalSymlinks(cleanOut)
	if err != nil {
		return "", fmt.Errorf("failed to resolve real path for output directory: %w", err)
	}

	rel, err := filepath.Rel(realDesktop, realOut)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("security restriction: GUI output directory must reside within ~/Desktop (got: %s)", cleanOut)
	}

	return realOut, nil
}

func validateReportName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "RRFSv2_Verification_Report", nil
	}
	if strings.ContainsAny(name, "/\\:*?\"<>|\r\n\x00") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid report name '%s': report name cannot contain path separators, control characters, or '..'", name)
	}
	for _, ext := range []string{".pptx", ".html", ".ppt"} {
		if strings.HasSuffix(strings.ToLower(name), ext) {
			name = name[:len(name)-len(ext)]
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "RRFSv2_Verification_Report", nil
	}
	return name, nil
}

func isPathAllowedToOpen(targetPath string) bool {
	if targetPath == "" {
		return false
	}
	clean := filepath.Clean(targetPath)
	if real, err := filepath.EvalSymlinks(clean); err == nil {
		clean = real
	}
	allowedPathsLock.RLock()
	allowed := allowedPathsMap[clean]
	allowedPathsLock.RUnlock()
	if allowed {
		return true
	}
	// Also allow if it is a directory strictly inside ~/Desktop
	if fi, err := os.Stat(clean); err == nil && fi.IsDir() {
		if _, err := validateDesktopDir(clean); err == nil {
			return true
		}
	}
	return false
}

func chooseFolderNative() string {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "Select Output Folder on Desktop" default location (path to desktop folder))`)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	} else if runtime.GOOS == "windows" {
		psScript := `Add-Type -AssemblyName System.Windows.Forms; $f = New-Object System.Windows.Forms.FolderBrowserDialog; if ($f.ShowDialog() -eq 'OK') { $f.SelectedPath }`
		cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return ""
}

func openPath(targetPath string) {
	if targetPath == "" {
		return
	}
	if !isPathAllowedToOpen(targetPath) {
		fmt.Printf("[SECURITY BLOCKED] Disallowed path open attempt: %s\n", targetPath)
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetPath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", targetPath)
	default:
		cmd = exec.Command("xdg-open", targetPath)
	}
	_ = cmd.Start()
}

func main() {
	// Default output directory: dynamically resolve current user's Desktop directory
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	defaultOut := filepath.Join(home, "Desktop")
	if !strings.HasSuffix(defaultOut, string(filepath.Separator)) {
		defaultOut += string(filepath.Separator)
	}

	renderedHTML := strings.ReplaceAll(uiHTML, "__DEFAULT_OUTPUT_DIR__", defaultOut)

	w := webview.New(false)
	defer w.Destroy()

	initMacMenu()

	w.SetTitle("NOAA GSL Verification Studio")
	w.SetSize(1160, 800, webview.HintNone)

	defer func() {
		pipelineMutex.Lock()
		if pipelineCancel != nil {
			pipelineCancel()
		}
		pipelineMutex.Unlock()
	}()

	// Bind native helper methods
	_ = w.Bind("quitApp", func() {
		pipelineMutex.Lock()
		if pipelineCancel != nil {
			pipelineCancel()
		}
		pipelineMutex.Unlock()
		w.Destroy()
		os.Exit(0)
	})

	_ = w.Bind("selectFolderDialog", func() string {
		return chooseFolderNative()
	})

	_ = w.Bind("openPathInOS", func(p string) {
		openPath(p)
	})

	_ = w.Bind("stopPipelineNative", func() {
		pipelineMutex.Lock()
		if isPipelineRunning && !isPipelineStopping && pipelineCancel != nil {
			isPipelineStopping = true
			cancel := pipelineCancel
			pipelineMutex.Unlock()
			b, _ := json.Marshal("\n[USER ACTION] ⏹ Stop request received. Terminating verification pipeline...\n")
			w.Dispatch(func() {
				w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
			})
			cancel()
		} else {
			pipelineMutex.Unlock()
		}
	})

	_ = w.Bind("runPipelineNative", func(payloadJSON string) {
		pipelineMutex.Lock()
		if isPipelineRunning {
			pipelineMutex.Unlock()
			b, _ := json.Marshal("\n[WARNING] Verification pipeline is already running.\n")
			w.Dispatch(func() {
				w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
			})
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		pipelineCancel = cancel
		isPipelineRunning = true
		isPipelineStopping = false
		pipelineMutex.Unlock()

		var req PipelineRequest
		if err := json.Unmarshal([]byte(payloadJSON), &req); err != nil {
			pipelineMutex.Lock()
			isPipelineRunning = false
			pipelineCancel = nil
			pipelineMutex.Unlock()
			b, _ := json.Marshal(fmt.Sprintf("\n[ERROR] Failed to parse input parameters: %v\n", err))
			w.Dispatch(func() {
				w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
				w.Eval("onPipelineFinished(false, 'Parameter parsing error')")
			})
			return
		}

		go func() {
			// 1. Strict security validation: Output directory MUST be strictly within ~/Desktop
			validOutDir, err := validateDesktopDir(req.OutputDir)
			if err != nil {
				pipelineMutex.Lock()
				isPipelineRunning = false
				pipelineCancel = nil
				pipelineMutex.Unlock()
				errMsg := fmt.Sprintf("\n[SECURITY ERROR] %v\n", err)
				b, _ := json.Marshal(errMsg)
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
					w.Eval("onPipelineFinished(false, 'Output directory must be within ~/Desktop')")
				})
				return
			}

			// 2. Strict security validation: Report name cannot contain traversal or path separators
			rawName := req.ReportName
			if rawName == "" {
				rawName = req.PptxName
			}
			baseName, err := validateReportName(rawName)
			if err != nil {
				pipelineMutex.Lock()
				isPipelineRunning = false
				pipelineCancel = nil
				pipelineMutex.Unlock()
				errMsg := fmt.Sprintf("\n[SECURITY ERROR] %v\n", err)
				b, _ := json.Marshal(errMsg)
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
					w.Eval("onPipelineFinished(false, 'Invalid report name')")
				})
				return
			}

			pptxPath := filepath.Join(validOutDir, baseName+".pptx")
			htmlPath := filepath.Join(validOutDir, baseName+".html")

			// Authorize UI to open these specific paths
			allowOpenPath(validOutDir)
			allowOpenPath(pptxPath)
			allowOpenPath(htmlPath)

			reportTitle := strings.TrimSpace(req.Title)
			if reportTitle == "" {
				reportTitle = "RRFSv2 Model Verification & Performance Evaluation"
			}

			sendOutput := func(text string) {
				b, _ := json.Marshal(text)
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
				})
			}

			pipeReq := pipeline.Request{
				Title:       reportTitle,
				OutputDir:   validOutDir,
				ReportName:  baseName,
				ModelsText:  req.ModelsText,
				SfcDates:    req.SfcDates,
				UaDates:     req.UaDates,
				Regions:     req.Regions,
				SkipFetch:   req.SkipFetch,
				ForceFetch:  req.ForceFetch,
				IncludeCref: req.IncludeCref,
				RadarCases:  req.RadarCases,
			}

			pipeErr := pipeline.Run(ctx, pipeReq, sendOutput)

			pipelineMutex.Lock()
			wasStopped := isPipelineStopping
			isPipelineRunning = false
			isPipelineStopping = false
			pipelineCancel = nil
			pipelineMutex.Unlock()

			if wasStopped {
				sendOutput(fmt.Sprintf("\n[STOPPED] ⏹ Verification pipeline was stopped by user.\n"))
				w.Dispatch(func() {
					w.Eval("onPipelineFinished(false, 'Pipeline stopped by user')")
				})
			} else if pipeErr != nil {
				sendOutput(fmt.Sprintf("\n[ERROR] Execution interrupted or exited with error: %v\n", pipeErr))
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("onPipelineFinished(false, 'Execution error: %v')", pipeErr))
				})
			} else {
				sendOutput(fmt.Sprintf("\n[SUCCESS] Verification pipeline completed successfully! Synchronized PPT and HTML presentations generated:\n  • PPT : %s\n  • HTML: %s\n", pptxPath, htmlPath))
				w.Dispatch(func() {
					w.Eval("onPipelineFinished(true, 'Generation complete')")
				})
			}
		}()
	})

	w.SetHtml(renderedHTML)
	w.Run()
}
