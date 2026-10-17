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

type thresholdSpec struct {
	MATSValue string
	Tag       string
	Label     string
}

// --------------------------------------------------------------------------
// Composite Reflectivity
// --------------------------------------------------------------------------

var radarThresholds = []thresholdSpec{
	{MATSValue: "15 (reflectivity >= 15 dBZ)", Tag: "15dbz", Label: "15 dBZ"},
	{MATSValue: "25 (reflectivity >= 25 dBZ)", Tag: "25dbz", Label: "25 dBZ"},
	{MATSValue: "35 (reflectivity >= 35 dBZ)", Tag: "35dbz", Label: "35 dBZ"},
}

// FetchReflectivity retrieves radar composite reflectivity verification (Dieoff & 1h/3h TS for 15, 25, 35 dBZ).
func (c *Client) FetchReflectivity(
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

	validModels, err := c.CheckAvailableModels(tabCtx, "https://gsl.noaa.gov/mats/radar", models, CheckOptions{
		Variable: "Composite Reflectivity",
	})
	if err != nil {
		c.Log("  [RADAR WARNING] Failed to connect to MATS Radar: %v. Skipping radar verification.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [RADAR WARNING] None of the requested models exist in MATS Radar database. Skipping radar verification.\n")
		return nil
	}

	// For each region: 3 dieoff (CSI+Bias) + 2 fhr * 3 ts (CSI+Bias) = 18 charts per region
	totalSteps := len(regions) * (len(radarThresholds)*2 + 2*len(radarThresholds)*2)
	currentStep := 0

	for _, reg := range regions {
		regTag := ""
		if reg != "all" {
			regTag = "_" + reg
		}
		regLabel := RadarRegionLabels[reg]
		if regLabel == "" {
			regLabel = "Continental US"
		}

		// 1. Dieoff: CSI & Bias
		for _, th := range radarThresholds {
			// CSI
			currentStep++
			fnCSI := fmt.Sprintf("ref_dieoff_csi_%s%s.png", th.Tag, regTag)
			outCSI := filepath.Join(outDir, fnCSI)
			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Radar CSI Dieoff (%s): %s", reg, th.Label))
			}
			if forceFetch || !isFileValid(outCSI) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fnCSI, 2, 2*time.Second, func() error {
					return c.fetchSingleRadarChart("Dieoff", regLabel, th.MATSValue, "CSI (Critical Success Index)", "", sfcDates, colorMap, validModels, outCSI)
				}); err != nil {
					c.Log("  [RADAR WARNING] Failed to fetch Radar CSI Dieoff (%s, %s): %v. Slot left empty.\n", th.Label, reg, err)
					_ = os.Remove(outCSI)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fnCSI)
			}

			// Bias
			currentStep++
			fnBias := fmt.Sprintf("ref_dieoff_bias_%s%s.png", th.Tag, regTag)
			outBias := filepath.Join(outDir, fnBias)
			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Radar Bias Dieoff (%s): %s", reg, th.Label))
			}
			if forceFetch || !isFileValid(outBias) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fnBias, 2, 2*time.Second, func() error {
					return c.fetchSingleRadarChart("Dieoff", regLabel, th.MATSValue, "Bias (Forecast / Actual)", "", sfcDates, colorMap, validModels, outBias)
				}); err != nil {
					c.Log("  [RADAR WARNING] Failed to fetch Radar Bias Dieoff (%s, %s): %v. Slot left empty.\n", th.Label, reg, err)
					_ = os.Remove(outBias)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fnBias)
			}
		}

		// 2. Time Series: 1h & 3h (CSI & Bias)
		for _, fhr := range []string{"1", "3"} {
			for _, th := range radarThresholds {
				// TS CSI
				currentStep++
				fnCSI := fmt.Sprintf("ref_ts_%sh_csi_%s%s.png", fhr, th.Tag, regTag)
				outCSI := filepath.Join(outDir, fnCSI)
				if onProgress != nil {
					onProgress(currentStep, totalSteps, fmt.Sprintf("Radar %sh TS CSI (%s): %s", fhr, reg, th.Label))
				}
				if forceFetch || !isFileValid(outCSI) {
					time.Sleep(300 * time.Millisecond)
					if err := c.RetryFetch(fnCSI, 2, 2*time.Second, func() error {
						return c.fetchSingleRadarChart("TimeSeries", regLabel, th.MATSValue, "CSI (Critical Success Index)", fhr, sfcDates, colorMap, validModels, outCSI)
					}); err != nil {
						c.Log("  [RADAR WARNING] Failed to fetch Radar %sh TS CSI (%s, %s): %v. Slot left empty.\n", fhr, th.Label, reg, err)
						_ = os.Remove(outCSI)
					}
				} else {
					c.Log("  [CACHE HIT] Using cached: %s\n", fnCSI)
				}

				// TS Bias
				currentStep++
				fnBias := fmt.Sprintf("ref_ts_%sh_bias_%s%s.png", fhr, th.Tag, regTag)
				outBias := filepath.Join(outDir, fnBias)
				if onProgress != nil {
					onProgress(currentStep, totalSteps, fmt.Sprintf("Radar %sh TS Bias (%s): %s", fhr, reg, th.Label))
				}
				if forceFetch || !isFileValid(outBias) {
					time.Sleep(300 * time.Millisecond)
					if err := c.RetryFetch(fnBias, 2, 2*time.Second, func() error {
						return c.fetchSingleRadarChart("TimeSeries", regLabel, th.MATSValue, "Bias (Forecast / Actual)", fhr, sfcDates, colorMap, validModels, outBias)
					}); err != nil {
						c.Log("  [RADAR WARNING] Failed to fetch Radar %sh TS Bias (%s, %s): %v. Slot left empty.\n", fhr, th.Label, reg, err)
						_ = os.Remove(outBias)
					}
				} else {
					c.Log("  [CACHE HIT] Using cached: %s\n", fnBias)
				}
			}
		}
	}

	return nil
}

func (c *Client) fetchSingleRadarChart(
	plotType, regLabel, threshStr, stat, fhr, sfcDates string,
	colorMap map[string]string,
	models []Model,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/radar"); err != nil {
		return fmt.Errorf("failed to open MATS radar page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", plotType)); err != nil {
		return fmt.Errorf("failed to select %s: %w", plotType, err)
	}

	time.Sleep(600 * time.Millisecond)

	_ = chromedp.Do(tabCtx, chromedp.Evaluate[chromedp.Void](fmt.Sprintf("(() => { %s })()", SetParamJS("variable", "Composite Reflectivity"))))
	time.Sleep(600 * time.Millisecond)

	for _, m := range models {
		params := []string{
			SetParamJS("data-source", m.ID),
			SetParamJS("scale", "3 km grid"),
			SetParamJS("region", regLabel),
			SetParamJS("variable", "Composite Reflectivity"),
			SetParamJS("threshold", threshStr),
			SetParamJS("statistic", stat),
			SetParamJS("curve-dates", sfcDates),
		}
		if plotType == "Dieoff" {
			params = append(params, SetParamJS("dieoff-type", "Dieoff"))
		} else if plotType == "TimeSeries" {
			params = append(params, SetParamJS("forecast-length", fhr))
		}

		if err := chromedp.Do(tabCtx,
			chromedp.Evaluate[chromedp.Void](fmt.Sprintf("(() => { %s })()", strings.Join(params, "\n"))),
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

// --------------------------------------------------------------------------
// Cloud Ceiling
// --------------------------------------------------------------------------

var ceilingThresholds = []thresholdSpec{
	{MATSValue: "500 (ceiling <500 ft)", Tag: "500ft", Label: "500ft"},
	{MATSValue: "1000 (ceiling <1000 ft)", Tag: "1000ft", Label: "1000ft"},
	{MATSValue: "3000 (ceiling <3000 ft)", Tag: "3000ft", Label: "3000ft"},
}

// FetchCeiling retrieves cloud ceiling verification (Dieoff & 1h/3h TS for 500ft, 1000ft, 3000ft).
func (c *Client) FetchCeiling(
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

	validModels, err := c.CheckAvailableModels(tabCtx, "https://gsl.noaa.gov/mats/ceil-vis", models, CheckOptions{
		Variable: "Ceiling",
	})
	if err != nil {
		c.Log("  [CEILING NOTICE] Failed to query MATS Ceiling: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [CEILING WARNING] None of the requested models exist in MATS Ceiling database. Skipping.\n")
		return nil
	}

	totalSteps := len(regions) * (len(ceilingThresholds)*2 + 2*len(ceilingThresholds)*2)
	currentStep := 0

	for _, reg := range regions {
		regTag := ""
		if reg != "all" {
			regTag = "_" + reg
		}
		regLabel := CeilVisRegionLabels[reg]
		if regLabel == "" {
			regLabel = "All HRRR domain"
		}

		// 1. Dieoff: CSI & Bias
		for _, th := range ceilingThresholds {
			// CSI
			currentStep++
			fnCSI := fmt.Sprintf("ceil_dieoff_csi_%s%s.png", th.Tag, regTag)
			outCSI := filepath.Join(outDir, fnCSI)
			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Ceiling CSI Dieoff (%s): %s", reg, th.Label))
			}
			if forceFetch || !isFileValid(outCSI) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fnCSI, 2, 2*time.Second, func() error {
					return c.fetchSingleCeilVisChart("Dieoff", "Ceiling", regLabel, th.MATSValue, "CSI (Critical Success Index)", "", sfcDates, colorMap, validModels, outCSI)
				}); err != nil {
					c.Log("  [CEILING WARNING] Failed to fetch Ceiling CSI Dieoff (%s, %s): %v. Slot left empty.\n", th.Label, reg, err)
					_ = os.Remove(outCSI)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fnCSI)
			}

			// Bias
			currentStep++
			fnBias := fmt.Sprintf("ceil_dieoff_bias_%s%s.png", th.Tag, regTag)
			outBias := filepath.Join(outDir, fnBias)
			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Ceiling Bias Dieoff (%s): %s", reg, th.Label))
			}
			if forceFetch || !isFileValid(outBias) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fnBias, 2, 2*time.Second, func() error {
					return c.fetchSingleCeilVisChart("Dieoff", "Ceiling", regLabel, th.MATSValue, "Bias (Forecast / Actual)", "", sfcDates, colorMap, validModels, outBias)
				}); err != nil {
					c.Log("  [CEILING WARNING] Failed to fetch Ceiling Bias Dieoff (%s, %s): %v. Slot left empty.\n", th.Label, reg, err)
					_ = os.Remove(outBias)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fnBias)
			}
		}

		// 2. Time Series: 1h & 3h (CSI & Bias)
		for _, fhr := range []string{"1", "3"} {
			for _, th := range ceilingThresholds {
				// TS CSI
				currentStep++
				fnCSI := fmt.Sprintf("ceil_ts_%sh_csi_%s%s.png", fhr, th.Tag, regTag)
				outCSI := filepath.Join(outDir, fnCSI)
				if onProgress != nil {
					onProgress(currentStep, totalSteps, fmt.Sprintf("Ceiling %sh TS CSI (%s): %s", fhr, reg, th.Label))
				}
				if forceFetch || !isFileValid(outCSI) {
					time.Sleep(300 * time.Millisecond)
					if err := c.RetryFetch(fnCSI, 2, 2*time.Second, func() error {
						return c.fetchSingleCeilVisChart("TimeSeries", "Ceiling", regLabel, th.MATSValue, "CSI (Critical Success Index)", fhr, sfcDates, colorMap, validModels, outCSI)
					}); err != nil {
						c.Log("  [CEILING WARNING] Failed to fetch Ceiling %sh TS CSI (%s, %s): %v. Slot left empty.\n", fhr, th.Label, reg, err)
						_ = os.Remove(outCSI)
					}
				} else {
					c.Log("  [CACHE HIT] Using cached: %s\n", fnCSI)
				}

				// TS Bias
				currentStep++
				fnBias := fmt.Sprintf("ceil_ts_%sh_bias_%s%s.png", fhr, th.Tag, regTag)
				outBias := filepath.Join(outDir, fnBias)
				if onProgress != nil {
					onProgress(currentStep, totalSteps, fmt.Sprintf("Ceiling %sh TS Bias (%s): %s", fhr, reg, th.Label))
				}
				if forceFetch || !isFileValid(outBias) {
					time.Sleep(300 * time.Millisecond)
					if err := c.RetryFetch(fnBias, 2, 2*time.Second, func() error {
						return c.fetchSingleCeilVisChart("TimeSeries", "Ceiling", regLabel, th.MATSValue, "Bias (Forecast / Actual)", fhr, sfcDates, colorMap, validModels, outBias)
					}); err != nil {
						c.Log("  [CEILING WARNING] Failed to fetch Ceiling %sh TS Bias (%s, %s): %v. Slot left empty.\n", fhr, th.Label, reg, err)
						_ = os.Remove(outBias)
					}
				} else {
					c.Log("  [CACHE HIT] Using cached: %s\n", fnBias)
				}
			}
		}
	}

	return nil
}

// --------------------------------------------------------------------------
// Visibility
// --------------------------------------------------------------------------

var visThresholds = []thresholdSpec{
	{MATSValue: "1 (visibility < 1 mi)", Tag: "1mi", Label: "1mi"},
	{MATSValue: "3 (visibility < 3 mi)", Tag: "3mi", Label: "3mi"},
	{MATSValue: "5 (visibility < 5 mi)", Tag: "5mi", Label: "5mi"},
}

// FetchVisibility retrieves visibility verification (Dieoff & 1h/3h TS for 1mi, 3mi, 5mi).
func (c *Client) FetchVisibility(
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

	validModels, err := c.CheckAvailableModels(tabCtx, "https://gsl.noaa.gov/mats/ceil-vis", models, CheckOptions{
		Variable: "Visibility",
	})
	if err != nil {
		c.Log("  [VISIBILITY NOTICE] Failed to query MATS Visibility: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [VISIBILITY WARNING] None of the requested models exist in MATS Visibility database. Skipping.\n")
		return nil
	}

	totalSteps := len(regions) * (len(visThresholds)*2 + 2*len(visThresholds)*2)
	currentStep := 0

	for _, reg := range regions {
		regTag := ""
		if reg != "all" {
			regTag = "_" + reg
		}
		regLabel := CeilVisRegionLabels[reg]
		if regLabel == "" {
			regLabel = "All HRRR domain"
		}

		// 1. Dieoff: CSI & Bias
		for _, th := range visThresholds {
			// CSI
			currentStep++
			fnCSI := fmt.Sprintf("vis_dieoff_csi_%s%s.png", th.Tag, regTag)
			outCSI := filepath.Join(outDir, fnCSI)
			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Visibility CSI Dieoff (%s): %s", reg, th.Label))
			}
			if forceFetch || !isFileValid(outCSI) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fnCSI, 2, 2*time.Second, func() error {
					return c.fetchSingleCeilVisChart("Dieoff", "Visibility", regLabel, th.MATSValue, "CSI (Critical Success Index)", "", sfcDates, colorMap, validModels, outCSI)
				}); err != nil {
					c.Log("  [VISIBILITY WARNING] Failed to fetch Visibility CSI Dieoff (%s, %s): %v. Slot left empty.\n", th.Label, reg, err)
					_ = os.Remove(outCSI)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fnCSI)
			}

			// Bias
			currentStep++
			fnBias := fmt.Sprintf("vis_dieoff_bias_%s%s.png", th.Tag, regTag)
			outBias := filepath.Join(outDir, fnBias)
			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Visibility Bias Dieoff (%s): %s", reg, th.Label))
			}
			if forceFetch || !isFileValid(outBias) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fnBias, 2, 2*time.Second, func() error {
					return c.fetchSingleCeilVisChart("Dieoff", "Visibility", regLabel, th.MATSValue, "Bias (Forecast / Actual)", "", sfcDates, colorMap, validModels, outBias)
				}); err != nil {
					c.Log("  [VISIBILITY WARNING] Failed to fetch Visibility Bias Dieoff (%s, %s): %v. Slot left empty.\n", th.Label, reg, err)
					_ = os.Remove(outBias)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fnBias)
			}
		}

		// 2. Time Series: 1h & 3h (CSI & Bias)
		for _, fhr := range []string{"1", "3"} {
			for _, th := range visThresholds {
				// TS CSI
				currentStep++
				fnCSI := fmt.Sprintf("vis_ts_%sh_csi_%s%s.png", fhr, th.Tag, regTag)
				outCSI := filepath.Join(outDir, fnCSI)
				if onProgress != nil {
					onProgress(currentStep, totalSteps, fmt.Sprintf("Visibility %sh TS CSI (%s): %s", fhr, reg, th.Label))
				}
				if forceFetch || !isFileValid(outCSI) {
					time.Sleep(300 * time.Millisecond)
					if err := c.RetryFetch(fnCSI, 2, 2*time.Second, func() error {
						return c.fetchSingleCeilVisChart("TimeSeries", "Visibility", regLabel, th.MATSValue, "CSI (Critical Success Index)", fhr, sfcDates, colorMap, validModels, outCSI)
					}); err != nil {
						c.Log("  [VISIBILITY WARNING] Failed to fetch Visibility %sh TS CSI (%s, %s): %v. Slot left empty.\n", fhr, th.Label, reg, err)
						_ = os.Remove(outCSI)
					}
				} else {
					c.Log("  [CACHE HIT] Using cached: %s\n", fnCSI)
				}

				// TS Bias
				currentStep++
				fnBias := fmt.Sprintf("vis_ts_%sh_bias_%s%s.png", fhr, th.Tag, regTag)
				outBias := filepath.Join(outDir, fnBias)
				if onProgress != nil {
					onProgress(currentStep, totalSteps, fmt.Sprintf("Visibility %sh TS Bias (%s): %s", fhr, reg, th.Label))
				}
				if forceFetch || !isFileValid(outBias) {
					time.Sleep(300 * time.Millisecond)
					if err := c.RetryFetch(fnBias, 2, 2*time.Second, func() error {
						return c.fetchSingleCeilVisChart("TimeSeries", "Visibility", regLabel, th.MATSValue, "Bias (Forecast / Actual)", fhr, sfcDates, colorMap, validModels, outBias)
					}); err != nil {
						c.Log("  [VISIBILITY WARNING] Failed to fetch Visibility %sh TS Bias (%s, %s): %v. Slot left empty.\n", fhr, th.Label, reg, err)
						_ = os.Remove(outBias)
					}
				} else {
					c.Log("  [CACHE HIT] Using cached: %s\n", fnBias)
				}
			}
		}
	}

	return nil
}

func (c *Client) fetchSingleCeilVisChart(
	plotType, variable, regLabel, threshStr, stat, fhr, sfcDates string,
	colorMap map[string]string,
	models []Model,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/ceil-vis"); err != nil {
		return fmt.Errorf("failed to open MATS ceil-vis page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", plotType)); err != nil {
		return fmt.Errorf("failed to select %s: %w", plotType, err)
	}

	time.Sleep(600 * time.Millisecond)

	_ = chromedp.Do(tabCtx, chromedp.Evaluate[chromedp.Void](fmt.Sprintf("(() => { %s })()", SetParamJS("variable", variable))))
	time.Sleep(600 * time.Millisecond)

	for _, m := range models {
		params := []string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("variable", variable),
			SetParamJS("threshold", threshStr),
			SetParamJS("statistic", stat),
			SetParamJS("curve-dates", sfcDates),
		}
		if plotType == "Dieoff" {
			params = append(params, SetParamJS("dieoff-type", "Dieoff"))
		} else if plotType == "TimeSeries" {
			params = append(params, SetParamJS("forecast-length", fhr))
		}

		if err := chromedp.Do(tabCtx,
			chromedp.Evaluate[chromedp.Void](fmt.Sprintf("(() => { %s })()", strings.Join(params, "\n"))),
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
