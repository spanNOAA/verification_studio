package mats

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"verification_studio/browser"
)

// Client handles automated interaction with NOAA GSL MATS web applications.
type Client struct {
	session *browser.Session
}

// NewClient initializes a MATS automation client with a browser session.
func NewClient(session *browser.Session) *Client {
	return &Client{session: session}
}

// CheckAvailableModels navigates to a MATS page and verifies which models exist in the data-source dropdown.
func (c *Client) CheckAvailableModels(ctx context.Context, pageURL string, models []Model, opts ...CheckOptions) ([]Model, error) {
	if err := chromedp.Do(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible("#plotTypes-selector"),
	); err != nil {
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
				fmt.Printf("  [NOTICE] Model %s (%s) not found in MATS database at %s. Skipping.\n", m.Name, m.ID, pageURL)
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
				trace.line.width = 3;
			}
			if (trace.marker) trace.marker.color = color;
			if (trace.error_y) trace.error_y.color = color;
			if (trace.error_x) trace.error_x.color = color;

			if (isBias && hasMainMetric) {
				trace.showlegend = false;
			} else {
				trace.name = shortName;
				trace.showlegend = true;
			}
		});

		if (gd.layout) {
			// Enlarge chart title (v1 & v2 compatible)
			if (gd.layout.title) {
				if (typeof gd.layout.title === 'string') {
					gd.layout.titlefont = Object.assign({}, gd.layout.titlefont, { size: 26, color: '#0f172a' });
					gd.layout.title = { text: gd.layout.title, font: { size: 26, color: '#0f172a' } };
				} else {
					gd.layout.title.font = Object.assign({}, gd.layout.title.font, { size: 26, color: '#0f172a' });
				}
			} else if (gd.layout.titlefont) {
				gd.layout.titlefont.size = 26;
			}

			// Enlarge axes titles & tick fonts (v1 & v2 compatible)
			['xaxis', 'yaxis', 'xaxis2', 'yaxis2'].forEach(ax => {
				if (gd.layout[ax]) {
					if (gd.layout[ax].title) {
						if (typeof gd.layout[ax].title === 'string') {
							gd.layout[ax].titlefont = Object.assign({}, gd.layout[ax].titlefont, { size: 20, color: '#1e293b' });
							gd.layout[ax].title = { text: gd.layout[ax].title, font: { size: 20, color: '#1e293b' } };
						} else {
							gd.layout[ax].title.font = Object.assign({}, gd.layout[ax].title.font, { size: 20, color: '#1e293b' });
						}
					} else if (gd.layout[ax].titlefont) {
						gd.layout[ax].titlefont.size = 20;
					}
					if (gd.layout[ax].tickfont) {
						gd.layout[ax].tickfont.size = 16;
					} else {
						gd.layout[ax].tickfont = { size: 16, color: '#334155' };
					}
				}
			});

			if (gd.layout.legend) {
				gd.layout.legend.font = { size: 16, color: '#1e293b' };
				gd.layout.legend.bgcolor = 'rgba(255, 255, 255, 0.85)';
				gd.layout.legend.bordercolor = 'rgba(203, 213, 225, 0.8)';
				gd.layout.legend.borderwidth = 1;
			}
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
