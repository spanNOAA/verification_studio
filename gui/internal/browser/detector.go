package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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

// FindBrowser inspects the host system for an available Chromium-compatible browser.
// Priority: Google Chrome -> Microsoft Edge -> Brave -> Chromium -> Arc -> System PATH -> Portable Cache.
func FindBrowser() (*BrowserInfo, error) {
	if runtime.GOOS == "darwin" {
		homeDir, _ := os.UserHomeDir()
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

		// Check portable cache directory: ~/Library/Application Support/Verification Studio/bin/chrome-headless-shell
		portableDir := filepath.Join(homeDir, "Library", "Application Support", "Verification Studio", "bin")
		portablePath := filepath.Join(portableDir, "chrome-headless-shell")
		if isExecutable(portablePath) {
			return &BrowserInfo{
				Name:       "Chrome Headless Shell (Portable)",
				Path:       portablePath,
				IsPortable: true,
			}, nil
		}
	}

	// Fallback to system PATH lookup
	pathCandidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"microsoft-edge",
		"microsoft-edge-stable",
		"chromium",
		"chromium-browser",
		"brave-browser",
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
	// Check if executable bit is set
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
