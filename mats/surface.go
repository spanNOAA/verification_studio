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

// SurfaceVar describes an evaluated 2m or 10m surface meteorology variable.
type SurfaceVar struct {
	MATSLabel string
	Tag       string
}

var SurfaceVars = []SurfaceVar{
	{MATSLabel: "Temperature at 2m (°C)", Tag: "t2m"},
	{MATSLabel: "Dewpoint at 2m (°C)", Tag: "td2m"},
	{MATSLabel: "Wind Speed at 10m (m/s)", Tag: "wind10m"},
}

// FetchSurfaceDieoff retrieves all surface dieoff verification charts across the specified regions.
func (c *Client) FetchSurfaceDieoff(
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

	tabCtx, cancel := c.session.NewTab(60 * time.Second)
	defer cancel()

	validModels, err := c.CheckAvailableModels(tabCtx, "https://gsl.noaa.gov/mats/surface", models)
	if err != nil {
		c.Log("  [SURFACE WARNING] Failed to connect to MATS Surface: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [SURFACE WARNING] None of the requested models exist in MATS Surface database. Skipping.\n")
		return nil
	}

	totalSteps := len(regions) * len(SurfaceVars)
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

		for _, v := range SurfaceVars {
			currentStep++
			fn := fmt.Sprintf("surface_dieoff_%s%s.png", v.Tag, regTag)
			outPath := filepath.Join(outDir, fn)

			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Surface Dieoff (%s): %s", reg, v.MATSLabel))
			}

			if !forceFetch && isFileValid(outPath) {
				c.Log("  [CACHE HIT] Using cached: %s\n", fn)
				continue
			}

			time.Sleep(300 * time.Millisecond)
			if err := c.RetryFetch(fn, 2, 2*time.Second, func() error {
				return c.fetchSingleSurfaceDieoff(validModels, sfcDates, colorMap, regLabel, v.MATSLabel, outPath)
			}); err != nil {
				c.Log("  [SURFACE WARNING] Failed to fetch Surface Dieoff (%s, %s): %v. Slot left empty.\n", v.MATSLabel, reg, err)
				_ = os.Remove(outPath)
			}
		}
	}

	return nil
}

func (c *Client) fetchSingleSurfaceDieoff(
	models []Model,
	sfcDates string,
	colorMap map[string]string,
	regLabel string,
	varLabel string,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	// 1. Navigate and set Plot Type to Dieoff
	if err := chromedp.Do(tabCtx,
		chromedp.Navigate("https://gsl.noaa.gov/mats/surface"),
		chromedp.WaitVisible("#plotTypes-selector"),
		chromedp.SetValue("#plotTypes-selector", "Dieoff"),
	); err != nil {
		return fmt.Errorf("failed to open and select Dieoff: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	// 2. Add RMSE curves
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", "RMSE"),
			SetParamJS("variable", varLabel),
			SetParamJS("dieoff-type", "Dieoff"),
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

	// 3. Add Bias curves
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", "Bias (Model - Obs)"),
			SetParamJS("variable", varLabel),
			SetParamJS("dieoff-type", "Dieoff"),
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

	// 4. Plot and wait for rendering
	if err := chromedp.Do(tabCtx, chromedp.Click("#plotMatched")); err != nil {
		return fmt.Errorf("failed to click plotMatched: %w", err)
	}

	// Wait up to 15s for Plotly to render
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

	// 5. Export styled chart
	return c.ExportStyledChart(tabCtx, outPath, colorMap, models, 1761, 857)
}

func isFileValid(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() > 1000
}
