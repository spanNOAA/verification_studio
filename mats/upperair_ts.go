package mats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// UpperAirTSVar defines an evaluated upper air variable for time series.
type UpperAirTSVar struct {
	VarLabel string
	VarTag   string
}

// UpperAirTSVars defines standard upper air time series variables against RAOBs.
var UpperAirTSVars = []UpperAirTSVar{
	{VarLabel: "Temperature", VarTag: "temp"},
	{VarLabel: "Wind", VarTag: "wind"},
	{VarLabel: "RHobT", VarTag: "rh"},
}

// UpperAirTSStat defines an evaluated upper air statistical metric.
type UpperAirTSStat struct {
	Stat    string
	StatTag string
}

// UpperAirTSStats defines standard time series statistical metrics.
var UpperAirTSStats = []UpperAirTSStat{
	{Stat: "RMSE", StatTag: "rmse"},
	{Stat: "Bias (Model - Obs)", StatTag: "bias"},
}

// UpperAirTSConfig defines a single upper air time series chart query.
type UpperAirTSConfig struct {
	Fhr      string
	Stat     string
	StatTag  string
	VarLabel string
	VarTag   string
}

// BuildUpperAirTSConfigs builds time series configurations for requested upper air forecast leads.
func BuildUpperAirTSConfigs(leads []string) []UpperAirTSConfig {
	sortedLeads := SortLeadHours(leads)
	if len(sortedLeads) == 0 {
		return nil
	}
	var configs []UpperAirTSConfig
	for _, fhr := range sortedLeads {
		for _, st := range UpperAirTSStats {
			for _, v := range UpperAirTSVars {
				configs = append(configs, UpperAirTSConfig{
					Fhr:      fhr,
					Stat:     st.Stat,
					StatTag:  st.StatTag,
					VarLabel: v.VarLabel,
					VarTag:   v.VarTag,
				})
			}
		}
	}
	return configs
}

// FetchUpperAirTimeSeries retrieves upper air diurnal time series charts across regions and lead hours.
func (c *Client) FetchUpperAirTimeSeries(
	ctx context.Context,
	models []Model,
	uaDates string,
	colorMap map[string]string,
	outDir string,
	regions []string,
	leads []string,
	forceFetch bool,
	onProgress func(current, total int, message string),
) error {
	if len(regions) == 0 {
		regions = []string{"all"}
	}

	validModels, err := c.CheckAvailableModels(ctx, "https://gsl.noaa.gov/mats/upperair", models)
	if err != nil {
		c.Log("  [UPPER AIR TS WARNING] Failed to connect to MATS UpperAir: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [UPPER AIR TS WARNING] None of the requested models exist in MATS UpperAir database. Skipping.\n")
		return nil
	}

	configs := BuildUpperAirTSConfigs(leads)
	if len(configs) == 0 {
		c.Log("  [UPPER AIR TS NOTICE] No forecast lead hours selected. Skipping Upper Air Time Series.\n")
		return nil
	}

	totalSteps := len(regions) * len(configs)
	currentStep := 0

	for _, reg := range regions {
		regTag := ""
		if reg != "all" {
			regTag = "_" + reg
		}
		regLabel := UpperAirRegionLabels[reg]
		if regLabel == "" {
			regLabel = "HRRR domain"
		}

		for _, cfg := range configs {
			currentStep++
			fn := fmt.Sprintf("ua_ts_%sh_%s_%s%s.png", cfg.Fhr, cfg.StatTag, cfg.VarTag, regTag)
			outPath := filepath.Join(outDir, fn)

			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Upper Air TS %sh %s (%s): %s", cfg.Fhr, cfg.StatTag, reg, cfg.VarLabel))
			}

			if !forceFetch && isFileValid(outPath) {
				c.Log("  [CACHE HIT] Using cached: %s\n", fn)
				continue
			}

			time.Sleep(300 * time.Millisecond)
			if err := c.RetryFetch(fn, 2, 2*time.Second, func() error {
				return c.fetchSingleUpperAirTimeSeries(validModels, uaDates, colorMap, regLabel, cfg, outPath)
			}); err != nil {
				c.Log("  [UPPER AIR TS WARNING] Failed to fetch Upper Air TS (%s): %v. Slot left empty.\n", fn, err)
				_ = os.Remove(outPath)
			}
		}
	}

	return nil
}

func (c *Client) fetchSingleUpperAirTimeSeries(
	models []Model,
	uaDates string,
	colorMap map[string]string,
	regLabel string,
	cfg UpperAirTSConfig,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/upperair"); err != nil {
		return fmt.Errorf("failed to open MATS UpperAir page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", "TimeSeries")); err != nil {
		return fmt.Errorf("failed to select TimeSeries: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("database", "RAOBs (Traditional)"),
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", cfg.Stat),
			SetParamJS("variable", cfg.VarLabel),
			SetParamJS("forecast-length", cfg.Fhr),
			SetParamJS("top", "1"),
			SetParamJS("bottom", "1000"),
			SetParamJS("curve-dates", uaDates),
		}, "\n")

		if err := chromedp.Do(tabCtx,
			chromedp.Evaluate[chromedp.Void](fmt.Sprintf("(() => { %s })()", js)),
		); err != nil {
			return err
		}
		time.Sleep(150 * time.Millisecond)
		_ = chromedp.Do(tabCtx, chromedp.Click("#add"))
		time.Sleep(150 * time.Millisecond)
	}

	if err := chromedp.Do(tabCtx, chromedp.Click("#plotMatched")); err != nil {
		return fmt.Errorf("failed to click plotMatched: %w", err)
	}

	waitPlotScript := `(() => {
		const plots = Array.from(document.querySelectorAll('.js-plotly-plot'));
		const gd = plots.find(el => el.offsetWidth > 200 && el.offsetHeight > 200);
		return Boolean(gd && gd.data && gd.data.length > 0);
	})()`

	start := time.Now()
	rendered := false
	for time.Since(start) < 20*time.Second {
		ready, _ := chromedp.Run(tabCtx, chromedp.Evaluate[bool](waitPlotScript))
		if ready {
			rendered = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !rendered {
		return fmt.Errorf("timed out waiting for Plotly canvas to render")
	}

	time.Sleep(1500 * time.Millisecond)
	return c.ExportStyledChart(tabCtx, outPath, colorMap, models, 1761, 857)
}
