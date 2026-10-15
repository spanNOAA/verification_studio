package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/webview/webview_go"
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

func getExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	dir := filepath.Dir(exe)
	if strings.Contains(dir, ".app/Contents/MacOS") {
		dir = filepath.Clean(filepath.Join(dir, "../../.."))
	}
	return dir
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

func findPython3() string {
	exe, err := os.Executable()
	if err == nil {
		// 1. Check bundled Python inside .app Frameworks: .../Contents/Frameworks/python/bin/python3
		bundledPy := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "Frameworks", "python", "bin", "python3"))
		if _, err := os.Stat(bundledPy); err == nil {
			return bundledPy
		}
	}
	if venv := os.Getenv("VIRTUAL_ENV"); venv != "" {
		p := filepath.Join(venv, "bin", "python3")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	candidates := []string{
		"/usr/bin/python3",
		"/usr/local/bin/python3",
		"/opt/homebrew/bin/python3",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "miniconda3", "bin", "python3"),
			filepath.Join(home, "anaconda3", "bin", "python3"),
			filepath.Join(home, ".local", "bin", "python3"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if path, err := exec.LookPath("python3"); err == nil {
		return path
	}
	return "python3"
}

func findReporterScript(baseDir string) string {
	exe, err := os.Executable()
	if err == nil {
		// Inside macOS .app bundle: .../Contents/MacOS/mats_studio -> .../Contents/Resources/mats_reporter.py
		resScript := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "Resources", "mats_reporter.py"))
		if _, err := os.Stat(resScript); err == nil {
			return resScript
		}
	}
	candidates := []string{
		filepath.Join(baseDir, "mats_reporter.py"),
		filepath.Join(baseDir, "..", "mats_reporter.py"),
		"./mats_reporter.py",
		"../mats_reporter.py",
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	return "mats_reporter.py"
}

func main() {
	baseDir := getExecutableDir()

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

	// Bind native helper methods
	_ = w.Bind("quitApp", func() {
		w.Destroy()
		os.Exit(0)
	})

	_ = w.Bind("selectFolderDialog", func() string {
		return chooseFolderNative()
	})

	_ = w.Bind("openPathInOS", func(p string) {
		openPath(p)
	})

	_ = w.Bind("runPipelineNative", func(payloadJSON string) {
		var req PipelineRequest
		if err := json.Unmarshal([]byte(payloadJSON), &req); err != nil {
			b, _ := json.Marshal(fmt.Sprintf("\n[ERROR] Failed to parse input parameters: %v\n", err))
			w.Dispatch(func() {
				w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
				w.Eval("onPipelineFinished(false, 'Parameter parsing error')")
			})
			return
		}

		go func() {
			scriptPath := findReporterScript(baseDir)

			// 1. Strict security validation: Output directory MUST be strictly within ~/Desktop
			validOutDir, err := validateDesktopDir(req.OutputDir)
			if err != nil {
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
				errMsg := fmt.Sprintf("\n[SECURITY ERROR] %v\n", err)
				b, _ := json.Marshal(errMsg)
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
					w.Eval("onPipelineFinished(false, 'Invalid report name')")
				})
				return
			}

			// Parse models: pass each non-empty trimmed line directly to python script
			rawLines := strings.Split(req.ModelsText, "\n")
			var modelArgs []string
			for _, line := range rawLines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				modelArgs = append(modelArgs, line)
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

			args := []string{
				scriptPath,
				"--title", reportTitle,
				"--output-dir", validOutDir,
				"--report-name", baseName,
				"--output-pptx", pptxPath,
				"--output-html", htmlPath,
				"--surface-dates", req.SfcDates,
				"--upperair-dates", req.UaDates,
			}

			if len(modelArgs) > 0 {
				args = append(args, "--models")
				args = append(args, modelArgs...)
			}

			if req.SkipFetch {
				args = append(args, "--skip-fetch")
			}

			if req.ForceFetch {
				args = append(args, "--force-fetch")
			}

			if req.IncludeCref {
				crefDir := filepath.Clean(filepath.Join(validOutDir, "cref"))
				refDir := filepath.Clean(filepath.Join(validOutDir, "extracted_retv4"))
				_ = os.MkdirAll(crefDir, 0o755)
				_ = os.MkdirAll(refDir, 0o755)
				args = append(args, "--cref-dir", crefDir)
				args = append(args, "--extracted-ref-dir", refDir)

				if len(req.RadarCases) > 0 {
					rcStr := strings.TrimSpace(string(req.RadarCases))
					if rcStr != "" && rcStr != "null" {
						args = append(args, "--radar-cases", rcStr)
					}
				}
			} else {
				args = append(args, "--skip-cref")
			}

			if len(req.Regions) > 0 {
				args = append(args, "--regions")
				for _, r := range req.Regions {
					args = append(args, r)
				}
			}

			sendOutput := func(text string) {
				b, _ := json.Marshal(text)
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("appendLog(%s)", string(b)))
				})
			}

			sendOutput(fmt.Sprintf("==================================================================\n"))
			sendOutput(fmt.Sprintf(" [Go Native Engine] Launching Python verification pipeline...\n"))
			sendOutput(fmt.Sprintf(" Script  : %s\n", scriptPath))
			sendOutput(fmt.Sprintf(" Title   : %s\n", req.Title))
			sendOutput(fmt.Sprintf(" Output  : %s\n", validOutDir))
			sendOutput(fmt.Sprintf(" Report  : %s (.pptx / .html)\n", baseName))
			sendOutput(fmt.Sprintf(" Regions : %s\n", strings.Join(req.Regions, ", ")))
			sendOutput(fmt.Sprintf(" PPTX    : %s\n", pptxPath))
			sendOutput(fmt.Sprintf(" HTML    : %s\n", htmlPath))
			pythonExe := findPython3()
			sendOutput(fmt.Sprintf(" Python  : %s\n", pythonExe))
			sendOutput(fmt.Sprintf("==================================================================\n\n"))

			cmd := exec.Command(pythonExe, args...)
			env := append(os.Environ(),
				"PYTHONUNBUFFERED=1",
				"HPC_NON_INTERACTIVE=1",
				"PATH="+filepath.Dir(pythonExe)+":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:"+os.Getenv("PATH"),
			)
			hpcRunPath := filepath.Clean(filepath.Join(filepath.Dir(pythonExe), "hpc-run"))
			if _, err := os.Stat(hpcRunPath); err == nil {
				env = append(env, "HPC_RUN_BIN="+hpcRunPath)
			}
			if exe, err := os.Executable(); err == nil {
				browsersDir := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "Frameworks", "browsers"))
				if _, err := os.Stat(browsersDir); err == nil {
					env = append(env, "PLAYWRIGHT_BROWSERS_PATH="+browsersDir)
				}
			}
			cmd.Env = env

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				sendOutput(fmt.Sprintf("\n[ERROR] Failed to create stdout pipe: %v\n", err))
				w.Dispatch(func() { w.Eval("onPipelineFinished(false, 'Failed to create stdout pipe')") })
				return
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				sendOutput(fmt.Sprintf("\n[ERROR] Failed to create stderr pipe: %v\n", err))
				w.Dispatch(func() { w.Eval("onPipelineFinished(false, 'Failed to create stderr pipe')") })
				return
			}

			if err := cmd.Start(); err != nil {
				sendOutput(fmt.Sprintf("\n[ERROR] Failed to start python process: %v\n", err))
				w.Dispatch(func() { w.Eval("onPipelineFinished(false, 'Failed to start process')") })
				return
			}

			// Stream stdout and stderr concurrently
			readStream := func(r io.Reader) {
				buf := make([]byte, 1024)
				for {
					n, err := r.Read(buf)
					if n > 0 {
						sendOutput(string(buf[:n]))
					}
					if err != nil {
						break
					}
				}
			}

			go readStream(stdout)
			go readStream(stderr)

			cmdErr := cmd.Wait()
			if cmdErr != nil {
				sendOutput(fmt.Sprintf("\n[ERROR] Execution interrupted or exited with error: %v\n", cmdErr))
				w.Dispatch(func() {
					w.Eval("onPipelineFinished(false, 'Execution error')")
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
