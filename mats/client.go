package mats

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"verification_studio/browser"
)

var (
	ErrRateLimited  = errors.New("rate limited by Cloudflare/gateway")
	ErrPageNotFound = errors.New("page returned HTTP 404 Not Found")
)

// Client handles automated interaction with NOAA GSL MATS web applications.
type Client struct {
	session       *browser.Session
	logger        func(string)
	mu            sync.Mutex
	cooldownCount int
}

// NewClient initializes a MATS automation client with a browser session.
func NewClient(session *browser.Session) *Client {
	return &Client{session: session}
}

// SetLogger configures an optional real-time logging sink for operational messages.
func (c *Client) SetLogger(fn func(string)) {
	c.logger = fn
}

// Log outputs a formatted operational message to the registered logger or standard output.
func (c *Client) Log(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if c.logger != nil {
		c.logger(msg)
	} else {
		fmt.Print(msg)
	}
}

// OpenMATSPage navigates to a MATS page and verifies the UI initializes cleanly.
// It fails fast if the page returns HTTP 404 or a Cloudflare challenge.
func (c *Client) OpenMATSPage(ctx context.Context, pageURL string) error {
	if err := chromedp.Do(ctx, chromedp.Navigate(pageURL)); err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", pageURL, err)
	}

	checkScript := `(() => {
		if (document.querySelector("#plotTypes-selector")) {
			return "ready";
		}
		const bodyText = (document.body ? document.body.innerText : "") || "";
		const title = document.title || "";
		if (bodyText.includes("404 page not found") || title.includes("404 Not Found") || bodyText.includes("404 Not Found")) {
			return "404";
		}
		if (title.includes("Just a moment...") || title.includes("Attention Required!") || bodyText.includes("Cloudflare")) {
			return "cloudflare";
		}
		return "loading";
	})()`

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		status, err := chromedp.Run(ctx, chromedp.Evaluate[string](checkScript))
		if err == nil {
			switch status {
			case "ready":
				return nil
			case "404":
				return ErrPageNotFound
			case "cloudflare":
				return ErrRateLimited
			}
		}

		if time.Since(start) > 25*time.Second {
			return fmt.Errorf("timed out waiting for MATS page controls to initialize")
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRateLimited) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "rate limited") || strings.Contains(s, "cloudflare") || strings.Contains(s, "429")
}

// CooldownAndRestart gracefully terminates the active browser session, sleeps for the specified duration
// while emitting real-time countdown notifications, and re-allocates a fresh browser session.
func (c *Client) CooldownAndRestart(ctx context.Context, duration time.Duration) error {
	c.mu.Lock()
	if c.cooldownCount >= 2 {
		c.mu.Unlock()
		c.Log("  [COOLDOWN NOTICE] Maximum rate limit cooldown attempts reached (%d). Skipping further pauses.\n", c.cooldownCount)
		return nil
	}
	c.cooldownCount++
	c.mu.Unlock()

	c.Log("\n[RATE LIMIT COOLDOWN] [!] Detected NOAA GSL / Cloudflare traffic throttling. Cooling down for %v...\n", duration)
	_ = c.session.Close()

	totalSec := int(duration.Seconds())
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	start := time.Now()
	for {
		elapsed := int(time.Since(start).Seconds())
		remaining := totalSec - elapsed
		if remaining <= 0 {
			break
		}
		c.Log("  [COOLDOWN] Remaining cooldown time: %ds...\n", remaining)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	c.Log("  [COOLDOWN] [OK] Cooldown completed. Re-initializing fresh browser session...\n\n")
	return c.session.Restart(ctx)
}

// RetryFetch executes a fetch action with automatic retry and backoff on transient network or timeout failures.
func (c *Client) RetryFetch(tag string, maxAttempts int, backoff time.Duration, fn func() error) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrPageNotFound) {
			// Permanent 404, no need to retry
			return err
		}
		if attempt < maxAttempts {
			if isRateLimited(err) {
				c.Log("  [RATE LIMIT] %s hit traffic throttling. Entering 3-minute cooldown...\n", tag)
				if cdErr := c.CooldownAndRestart(context.Background(), 3*time.Minute); cdErr != nil {
					c.Log("  [COOLDOWN ERROR] Failed to restart browser: %v\n", cdErr)
				}
			} else {
				c.Log("  [RETRY] %s encountered error (%v). Retrying in %v (attempt %d/%d)...\n", tag, err, backoff, attempt, maxAttempts)
				time.Sleep(backoff)
			}
		}
	}
	return err
}

// CheckAvailableModels navigates to a MATS page and verifies which models exist in the data-source dropdown.
func (c *Client) CheckAvailableModels(ctx context.Context, pageURL string, models []Model, opts ...CheckOptions) ([]Model, error) {
	if err := c.OpenMATSPage(ctx, pageURL); err != nil {
		return nil, fmt.Errorf("failed to open MATS page %s: %w", pageURL, err)
	}

	time.Sleep(1500 * time.Millisecond)

	var opt CheckOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	if opt.Database != "" {
		_ = chromedp.Do(ctx, chromedp.Evaluate[chromedp.Void](fmt.Sprintf(`(() => {
			if (window.matsParamUtils) {
				matsParamUtils.setInputValueForParamAndTriggerChange('database', '%s');
			}
		})()`, opt.Database)))
		time.Sleep(1000 * time.Millisecond)
	}

	if opt.Variable != "" {
		_ = chromedp.Do(ctx, chromedp.Evaluate[chromedp.Void](fmt.Sprintf(`(() => {
			if (window.matsParamUtils) {
				matsParamUtils.setInputValueForParamAndTriggerChange('variable', '%s');
			}
		})()`, opt.Variable)))
		time.Sleep(1000 * time.Millisecond)
	}

	var optionsJSON string
	script := `(() => {
		const sel = document.getElementById('data-source') || document.querySelector('select[name="data-source"]');
		if (!sel) return JSON.stringify([]);
		const opts = Array.from(sel.options).map(o => o.value || o.text);
		return JSON.stringify(opts);
	})()`

	var err error
	optionsJSON, err = chromedp.Run(ctx, chromedp.Evaluate[string](script))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve available models: %w", err)
	}

	var available []string
	if err := json.Unmarshal([]byte(optionsJSON), &available); err != nil {
		return nil, fmt.Errorf("failed to parse available options: %w", err)
	}

	availMap := make(map[string]bool)
	for _, a := range available {
		availMap[strings.TrimSpace(a)] = true
	}

	var valid []Model
	for _, m := range models {
		if availMap[m.ID] || availMap[m.Name] {
			valid = append(valid, m)
		} else {
			// Check case-insensitive
			found := false
			for a := range availMap {
				if strings.EqualFold(a, m.ID) || strings.EqualFold(a, m.Name) {
					valid = append(valid, m)
					found = true
					break
				}
			}
			if !found {
				c.Log("  [NOTICE] Model %s (%s) not found in MATS database at %s. Skipping.\n", m.Name, m.ID, pageURL)
			}
		}
	}

	return valid, nil
}

// ExportStyledChart applies uniform NOAA GSL line styles and exports the canvas directly to a PNG file.
func (c *Client) ExportStyledChart(ctx context.Context, outPath string, colorMap map[string]string, models []Model, width, height int) error {
	meta := map[string]any{
		"colorMap": colorMap,
		"models":   models,
	}
	metaBytes, _ := json.Marshal(meta)

	restyleScript := fmt.Sprintf(`(() => {
		const meta = %s;
		const colorMap = meta.colorMap || {};
		const models = meta.models || [];
		const plots = Array.from(document.querySelectorAll('.js-plotly-plot'));
		const gd = plots.find(el => el.offsetWidth > 200 && el.offsetHeight > 200);
		if (!gd || !gd.data) return false;

		const hasMainMetric = gd.data.some(t => {
			const n = (t.name || "").toLowerCase();
			return !n.includes("bias") && n !== "x = 0" && n !== "y = 0";
		});

		gd.data.forEach((trace) => {
			if (!trace.name || trace.name === "x = 0" || trace.name === "y = 0") return;

			let matchedModel = null;
			for (const m of models) {
				if (trace.name.includes(m.id) || trace.name.includes(m.name)) {
					matchedModel = m;
					break;
				}
			}

			let matchedColor = matchedModel ? matchedModel.color : null;
			if (!matchedColor) {
				for (const [key, color] of Object.entries(colorMap)) {
					if (trace.name.includes(key)) {
						matchedColor = color;
						break;
					}
				}
			}

			const isBias = trace.name.toLowerCase().includes("bias");
			const color = matchedColor || (trace.line && trace.line.color) || "#2563eb";
			const shortName = matchedModel ? matchedModel.name : trace.name.split(':')[0];

			if (trace.line) {
				trace.line.color = color;
				trace.line.dash = isBias ? "dash" : "solid";
				trace.line.width = 4.5;
			}
			if (trace.marker) {
				trace.marker.color = color;
				trace.marker.size = Math.max(trace.marker.size || 6, 8);
			}
			if (trace.error_y) {
				trace.error_y.color = color;
				trace.error_y.thickness = 2.5;
			}
			if (trace.error_x) {
				trace.error_x.color = color;
				trace.error_x.thickness = 2.5;
			}

			trace.name = shortName;
			trace.showlegend = false;
		});

		if (gd.layout) {
			// Clear chart title to maximize vertical plotting area
			gd.layout.title = { text: '' };
			if (gd.layout.titlefont) gd.layout.titlefont = { size: 0 };

			// Hide internal legend box to eliminate overlap (slide sidebar provides full legend)
			gd.layout.showlegend = false;
			if (gd.layout.legend) {
				delete gd.layout.legend;
			}

			// Compact top margin to grant extra vertical space for the curves
			if (!gd.layout.margin) gd.layout.margin = {};
			gd.layout.margin.t = 25;

			// Enlarge all axes titles & tick fonts across all subplots, subdomains, and secondary axes
			Object.keys(gd.layout).filter(k => /^[xy]axis\d*$/.test(k)).forEach(ax => {
				if (gd.layout[ax]) {
					gd.layout[ax].automargin = true;
					if (gd.layout[ax].title) {
						if (typeof gd.layout[ax].title === 'string') {
							gd.layout[ax].titlefont = Object.assign({}, gd.layout[ax].titlefont, { size: 34, color: '#0f172a' });
							gd.layout[ax].title = { text: gd.layout[ax].title, font: { size: 34, color: '#0f172a' } };
						} else {
							gd.layout[ax].title.font = Object.assign({}, gd.layout[ax].title.font, { size: 34, color: '#0f172a' });
						}
					} else if (gd.layout[ax].titlefont) {
						gd.layout[ax].titlefont.size = 34;
					}
					if (gd.layout[ax].tickfont) {
						gd.layout[ax].tickfont.size = 28;
						gd.layout[ax].tickfont.color = '#0f172a';
					} else {
						gd.layout[ax].tickfont = { size: 28, color: '#0f172a' };
					}
				}
			});
		}

		Plotly.redraw(gd);
		return true;
	})()`, string(metaBytes))

	_, err := chromedp.Run(ctx, chromedp.Evaluate[bool](restyleScript))
	if err != nil {
		return fmt.Errorf("failed to restyle Plotly chart: %w", err)
	}

	time.Sleep(800 * time.Millisecond)

	exportScript := fmt.Sprintf(`(async () => {
		const plots = Array.from(document.querySelectorAll('.js-plotly-plot'));
		const gd = plots.find(el => el.offsetWidth > 200 && el.offsetHeight > 200);
		if (window.Plotly && gd) {
			return await Plotly.toImage(gd, {
				format: 'png',
				width: %d,
				height: %d
			});
		}
		return "";
	})()`, width, height)

	imgData, err := chromedp.Run(ctx, chromedp.Evaluate[string](exportScript, chromedp.EvalAwaitPromise))
	if err != nil {
		return fmt.Errorf("failed to export Plotly canvas: %w", err)
	}

	if !strings.HasPrefix(imgData, "data:image/png;base64,") {
		return fmt.Errorf("invalid image data returned from Plotly.toImage: length=%d", len(imgData))
	}

	rawB64 := strings.TrimPrefix(imgData, "data:image/png;base64,")
	data, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 PNG data: %w", err)
	}

	// Atomic file write
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpFile := fmt.Sprintf("%s.tmp.%d", outPath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpFile, outPath)
}

// SetParamJS generates a JavaScript expression to set a MATS input parameter and trigger reactivity.
func SetParamJS(key, value string) string {
	escapedVal := strings.ReplaceAll(value, "'", "\\'")
	return fmt.Sprintf("matsParamUtils.setInputValueForParamAndTriggerChange('%s', '%s');", key, escapedVal)
}
