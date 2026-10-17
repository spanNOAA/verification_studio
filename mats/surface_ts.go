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

// SurfaceTSConfig defines a specific surface time series verification slice.
type SurfaceTSConfig struct {
	Fhr      string
	Stat     string
	StatTag  string
	VarLabel string
	VarTag   string
}

// SurfaceTSConfigs lists all standard diurnal cycle forecast lengths and metrics.
var SurfaceTSConfigs = []SurfaceTSConfig{
	{Fhr: "1", Stat: "RMSE", StatTag: "rmse", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "1", Stat: "RMSE", StatTag: "rmse", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "1", Stat: "RMSE", StatTag: "rmse", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},
	{Fhr: "1", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "1", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "1", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},

	{Fhr: "3", Stat: "RMSE", StatTag: "rmse", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "3", Stat: "RMSE", StatTag: "rmse", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "3", Stat: "RMSE", StatTag: "rmse", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},
	{Fhr: "3", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "3", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "3", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},

	{Fhr: "12", Stat: "RMSE", StatTag: "rmse", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "12", Stat: "RMSE", StatTag: "rmse", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "12", Stat: "RMSE", StatTag: "rmse", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},
	{Fhr: "12", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "12", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "12", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},

	{Fhr: "24", Stat: "RMSE", StatTag: "rmse", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "24", Stat: "RMSE", StatTag: "rmse", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "24", Stat: "RMSE", StatTag: "rmse", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},
	{Fhr: "24", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Temperature at 2m (°C)", VarTag: "t2m"},
	{Fhr: "24", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Dewpoint at 2m (°C)", VarTag: "td2m"},
	{Fhr: "24", Stat: "Bias (Model - Obs)", StatTag: "bias", VarLabel: "Wind Speed at 10m (m/s)", VarTag: "wind10m"},
}

// FetchSurfaceTimeSeries retrieves diurnal time series verification charts (1h, 3h, 12h, 24h) across regions.
func (c *Client) FetchSurfaceTimeSeries(
	ctx context.Context,
	models []Model,
	sfcDates string,
	colorMap map[string]string,
	outDir string,
	regions []string,
	forceFetch bool,
	onProgress func(current, total int, message string),
) error {
	if len(regions) == 0 {
		regions = []string{"all"}
	}

	validModels, err := c.CheckAvailableModels(ctx, "https://gsl.noaa.gov/mats/surface", models)
	if err != nil {
		c.Log("  [SURFACE TS WARNING] Failed to connect to MATS Surface: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [SURFACE TS WARNING] None of the requested models exist in MATS Surface database. Skipping.\n")
		return nil
	}

	totalSteps := len(regions) * len(SurfaceTSConfigs)
	currentStep := 0

	for _, reg := range regions {
		regTag := ""
		if reg != "all" {
			regTag = "_" + reg
		}
		regLabel := SurfaceRegionLabels[reg]
		if regLabel == "" {
			regLabel = "All HRRR domain"
		}

		for _, cfg := range SurfaceTSConfigs {
			currentStep++
			fn := fmt.Sprintf("sfc_ts_%sh_%s_%s%s.png", cfg.Fhr, cfg.StatTag, cfg.VarTag, regTag)
			outPath := filepath.Join(outDir, fn)

			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Surface TS %sh %s (%s): %s", cfg.Fhr, cfg.StatTag, reg, cfg.VarTag))
			}

			if !forceFetch && isFileValid(outPath) {
				c.Log("  [CACHE HIT] Using cached: %s\n", fn)
				continue
			}

			time.Sleep(300 * time.Millisecond)
			if err := c.RetryFetch(fn, 2, 2*time.Second, func() error {
				return c.fetchSingleSurfaceTimeSeries(validModels, sfcDates, colorMap, regLabel, cfg, outPath)
			}); err != nil {
				c.Log("  [SURFACE TS WARNING] Failed to fetch Surface TS (%s): %v. Slot left empty.\n", fn, err)
				_ = os.Remove(outPath)
			}
		}
	}

	return nil
}

func (c *Client) fetchSingleSurfaceTimeSeries(
	models []Model,
	sfcDates string,
	colorMap map[string]string,
	regLabel string,
	cfg SurfaceTSConfig,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/surface"); err != nil {
		return fmt.Errorf("failed to open MATS surface page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", "TimeSeries")); err != nil {
		return fmt.Errorf("failed to select TimeSeries: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", cfg.Stat),
			SetParamJS("variable", cfg.VarLabel),
			SetParamJS("forecast-length", cfg.Fhr),
			SetParamJS("truth", "METAR"),
			SetParamJS("curve-dates", sfcDates),
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
