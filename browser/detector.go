package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chromedp/chromedp"
)

// BrowserInfo contains metadata about a discovered Chromium browser.
type BrowserInfo struct {
	Name       string
	Path       string
	IsPortable bool
}

type candidate struct {
	name string
	rel  string
}

var macCandidates = []candidate{
	{name: "Google Chrome", rel: "Google Chrome.app/Contents/MacOS/Google Chrome"},
	{name: "Microsoft Edge", rel: "Microsoft Edge.app/Contents/MacOS/Microsoft Edge"},
	{name: "Brave Browser", rel: "Brave Browser.app/Contents/MacOS/Brave Browser"},
	{name: "Chromium", rel: "Chromium.app/Contents/MacOS/Chromium"},
	{name: "Arc", rel: "Arc.app/Contents/MacOS/Arc"},
}

var winCandidates = []candidate{
	{name: "Google Chrome", rel: filepath.Join("Google", "Chrome", "Application", "chrome.exe")},
	{name: "Microsoft Edge", rel: filepath.Join("Microsoft", "Edge", "Application", "msedge.exe")},
	{name: "Brave Browser", rel: filepath.Join("BraveSoftware", "Brave-Browser", "Application", "brave.exe")},
	{name: "Chromium", rel: filepath.Join("Chromium", "Application", "chrome.exe")},
}

var linuxCandidates = []candidate{
	{name: "Google Chrome", rel: "/usr/bin/google-chrome"},
	{name: "Google Chrome (Stable)", rel: "/usr/bin/google-chrome-stable"},
	{name: "Chromium", rel: "/usr/bin/chromium"},
	{name: "Chromium Browser", rel: "/usr/bin/chromium-browser"},
	{name: "Microsoft Edge", rel: "/usr/bin/microsoft-edge-stable"},
	{name: "Microsoft Edge (Dev)", rel: "/usr/bin/microsoft-edge"},
	{name: "Brave Browser", rel: "/usr/bin/brave-browser"},
	{name: "Chromium (Snap)", rel: "/snap/bin/chromium"},
}

// FindBrowser inspects the host system for an available Chromium-compatible browser.
// Priority: Native OS standard locations -> Portable Cache -> System PATH lookup.
func FindBrowser() (*BrowserInfo, error) {
	homeDir, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		appDirs := []string{
			"/Applications",
			filepath.Join(homeDir, "Applications"),
		}
		for _, cand := range macCandidates {
			for _, base := range appDirs {
				full := filepath.Join(base, cand.rel)
				if isExecutable(full) {
					return &BrowserInfo{
						Name:       cand.name,
						Path:       full,
						IsPortable: false,
					}, nil
				}
			}
		}

		portableDir := filepath.Join(homeDir, "Library", "Application Support", "Verification Studio", "bin")
		portablePath := filepath.Join(portableDir, "chrome-headless-shell")
		if isExecutable(portablePath) {
			return &BrowserInfo{
				Name:       "Chrome Headless Shell (Portable)",
				Path:       portablePath,
				IsPortable: true,
			}, nil
		}

	case "windows":
		var searchBases []string
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			searchBases = append(searchBases, pf)
		}
		if pfx86 := os.Getenv("ProgramFiles(x86)"); pfx86 != "" {
			searchBases = append(searchBases, pfx86)
		}
		if localApp := os.Getenv("LocalAppData"); localApp != "" {
			searchBases = append(searchBases, localApp)
		}

		for _, cand := range winCandidates {
			for _, base := range searchBases {
				full := filepath.Join(base, cand.rel)
				if isExecutable(full) {
					return &BrowserInfo{
						Name:       cand.name,
						Path:       full,
						IsPortable: false,
					}, nil
				}
			}
		}

		if localApp := os.Getenv("LocalAppData"); localApp != "" {
			portablePath := filepath.Join(localApp, "Verification Studio", "bin", "chrome-headless-shell.exe")
			if isExecutable(portablePath) {
				return &BrowserInfo{
					Name:       "Chrome Headless Shell (Portable)",
					Path:       portablePath,
					IsPortable: true,
				}, nil
			}
		}

	case "linux":
		for _, cand := range linuxCandidates {
			if isExecutable(cand.rel) {
				return &BrowserInfo{
					Name:       cand.name,
					Path:       cand.rel,
					IsPortable: false,
				}, nil
			}
		}

		portableDir := filepath.Join(homeDir, ".local", "share", "verification-studio", "bin")
		portablePath := filepath.Join(portableDir, "chrome-headless-shell")
		if isExecutable(portablePath) {
			return &BrowserInfo{
				Name:       "Chrome Headless Shell (Portable)",
				Path:       portablePath,
				IsPortable: true,
			}, nil
		}
	}

	// Fallback to system PATH lookup for all platforms
	pathCandidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"microsoft-edge",
		"microsoft-edge-stable",
		"chromium",
		"chromium-browser",
		"brave-browser",
		"chrome.exe",
		"msedge.exe",
	}

	for _, name := range pathCandidates {
		if path, err := exec.LookPath(name); err == nil && isExecutable(path) {
			return &BrowserInfo{
				Name:       name,
				Path:       path,
				IsPortable: false,
			}, nil
		}
	}

	return nil, fmt.Errorf("no Chromium browser (Google Chrome, Microsoft Edge, Brave, Chromium) found on system")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".exe" || ext == ".cmd" || ext == ".bat"
	}
	return info.Mode()&0o111 != 0
}

// DefaultExecAllocatorOptions returns standard headless allocator flags optimized for MATS Plotly canvas rendering.
func DefaultExecAllocatorOptions(execPath string, headless bool) []chromedp.ExecAllocatorOption {
	opts := append(chromedp.DefaultExecAllocatorOptions[:0],
		chromedp.ExecPath(execPath),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"),
	)

	if headless {
		opts = append(opts, chromedp.Flag("headless", "new"))
	}

	return opts
}
