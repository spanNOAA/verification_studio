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

// UADieoffVar defines an evaluated upper air meteorology variable against RAOBs.
type UADieoffVar struct {
	MATSLabel string
	Tag       string
}

var UADieoffVars = []UADieoffVar{
	{MATSLabel: "Temperature", Tag: "temp"},
	{MATSLabel: "RHobT", Tag: "rh"},
	{MATSLabel: "Wind", Tag: "wind"},
}

// FetchUpperAirDieoff retrieves upper air dieoff verification against RAOBs across regions.
func (c *Client) FetchUpperAirDieoff(
	ctx context.Context,
	models []Model,
	uaDates string,
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

	validModels, err := c.CheckAvailableModels(tabCtx, "https://gsl.noaa.gov/mats/upperair", models)
	if err != nil {
		c.Log("  [UPPER AIR NOTICE] Failed to connect to MATS UpperAir: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [UPPER AIR WARNING] None of the requested models exist in MATS UpperAir database. Skipping.\n")
		return nil
	}

	totalSteps := len(regions) * len(UADieoffVars)
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

		for _, v := range UADieoffVars {
			currentStep++
			fn := fmt.Sprintf("ua_dieoff_%s%s.png", v.Tag, regTag)
			outPath := filepath.Join(outDir, fn)

			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Upper Air Dieoff (%s): %s", reg, v.MATSLabel))
			}

			if forceFetch || !isFileValid(outPath) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fn, 2, 2*time.Second, func() error {
					return c.fetchSingleUADieoff(validModels, uaDates, colorMap, regLabel, v.MATSLabel, outPath)
				}); err != nil {
					c.Log("  [UPPER AIR WARNING] Failed to fetch UA Dieoff (%s, %s): %v. Slot left empty.\n", v.MATSLabel, reg, err)
					_ = os.Remove(outPath)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fn)
			}
		}
	}

	return nil
}

func (c *Client) fetchSingleUADieoff(
	models []Model,
	uaDates string,
	colorMap map[string]string,
	regLabel string,
	varLabel string,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/upperair"); err != nil {
		return fmt.Errorf("failed to open MATS UpperAir page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", "Dieoff")); err != nil {
		return fmt.Errorf("failed to select Dieoff: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	// 1. Add RMSE curves
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", "RMSE"),
			SetParamJS("variable", varLabel),
			SetParamJS("dieoff-type", "Dieoff"),
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

	// 2. Add Bias curves
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", "Bias (Model - Obs)"),
			SetParamJS("variable", varLabel),
			SetParamJS("dieoff-type", "Dieoff"),
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

// UAProfileConfig defines vertical profile configurations.
type UAProfileConfig struct {
	Fhr      string
	VarLabel string
	VarTag   string
}

var UAProfileConfigs = []UAProfileConfig{
	{Fhr: "1", VarLabel: "Temperature", VarTag: "temp"},
	{Fhr: "1", VarLabel: "Wind", VarTag: "wind"},
	{Fhr: "1", VarLabel: "RHobT", VarTag: "rh"},

	{Fhr: "3", VarLabel: "Temperature", VarTag: "temp"},
	{Fhr: "3", VarLabel: "Wind", VarTag: "wind"},
	{Fhr: "3", VarLabel: "RHobT", VarTag: "rh"},

	{Fhr: "6", VarLabel: "Temperature", VarTag: "temp"},
	{Fhr: "6", VarLabel: "Wind", VarTag: "wind"},
	{Fhr: "6", VarLabel: "RHobT", VarTag: "rh"},

	{Fhr: "12", VarLabel: "Temperature", VarTag: "temp"},
	{Fhr: "12", VarLabel: "Wind", VarTag: "wind"},
	{Fhr: "12", VarLabel: "RHobT", VarTag: "rh"},
}

// FetchUpperAirProfiles retrieves vertical profile verification against RAOBs (1h, 3h, 6h, 12h).
func (c *Client) FetchUpperAirProfiles(
	ctx context.Context,
	models []Model,
	uaDates string,
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

	validModels, err := c.CheckAvailableModels(tabCtx, "https://gsl.noaa.gov/mats/upperair", models)
	if err != nil {
		c.Log("  [UPPER AIR PROFILE NOTICE] Failed to connect to MATS UpperAir: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [UPPER AIR WARNING] None of the requested models exist in MATS UpperAir database. Skipping.\n")
		return nil
	}

	totalSteps := len(regions) * len(UAProfileConfigs)
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

		for _, cfg := range UAProfileConfigs {
			currentStep++
			fn := fmt.Sprintf("ua_prof_%sh_%s%s.png", cfg.Fhr, cfg.VarTag, regTag)
			outPath := filepath.Join(outDir, fn)

			if onProgress != nil {
				onProgress(currentStep, totalSteps, fmt.Sprintf("Upper Air Profile %sh (%s): %s", cfg.Fhr, reg, cfg.VarLabel))
			}

			if forceFetch || !isFileValid(outPath) {
				time.Sleep(300 * time.Millisecond)
				if err := c.RetryFetch(fn, 2, 2*time.Second, func() error {
					return c.fetchSingleUAProfile(validModels, uaDates, colorMap, regLabel, cfg, outPath)
				}); err != nil {
					c.Log("  [UPPER AIR WARNING] Failed to fetch UA Profile (%s): %v. Slot left empty.\n", fn, err)
					_ = os.Remove(outPath)
				}
			} else {
				c.Log("  [CACHE HIT] Using cached: %s\n", fn)
			}
		}
	}

	return nil
}

func (c *Client) fetchSingleUAProfile(
	models []Model,
	uaDates string,
	colorMap map[string]string,
	regLabel string,
	cfg UAProfileConfig,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/upperair"); err != nil {
		return fmt.Errorf("failed to open MATS UpperAir page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", "Profile")); err != nil {
		return fmt.Errorf("failed to select Profile: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	// 1. RMSE
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", "RMSE"),
			SetParamJS("variable", cfg.VarLabel),
			SetParamJS("forecast-length", cfg.Fhr),
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

	// 2. Bias
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("data-source", m.ID),
			SetParamJS("region-type", "Predefined region"),
			SetParamJS("region", regLabel),
			SetParamJS("statistic", "Bias (Model - Obs)"),
			SetParamJS("variable", cfg.VarLabel),
			SetParamJS("forecast-length", cfg.Fhr),
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
	return c.ExportStyledChart(tabCtx, outPath, colorMap, models, 900, 900)
}

// FetchUpperAirAMDAR queries aircraft AMDAR observations for both Dieoff and Profiles.
func (c *Client) FetchUpperAirAMDAR(
	ctx context.Context,
	models []Model,
	uaDates string,
	colorMap map[string]string,
	outDir string,
	forceFetch bool,
	onProgress func(current, total int, message string),
) error {
	tabCtx, cancel := c.session.NewTab(60 * time.Second)
	defer cancel()

	validModels, err := c.CheckAvailableModels(tabCtx, "https://gsl.noaa.gov/mats/upperair", models, CheckOptions{
		Database: "AMDAR (Traditional)",
	})
	if err != nil {
		c.Log("  [AMDAR NOTICE] Could not query AMDAR database: %v. Gracefully skipping.\n", err)
		return nil
	}
	if len(validModels) == 0 {
		c.Log("  [AMDAR WARNING] None of the requested models exist in MATS AMDAR database. Skipping.\n")
		return nil
	}

	amdarDieoffVars := []UADieoffVar{
		{MATSLabel: "Temperature", Tag: "temp"},
		{MATSLabel: "Wind", Tag: "wind"},
		{MATSLabel: "RHobT", Tag: "rh"},
	}

	amdarProfConfigs := []UAProfileConfig{
		{Fhr: "1", VarLabel: "Temperature", VarTag: "temp"},
		{Fhr: "1", VarLabel: "Wind", VarTag: "wind"},
		{Fhr: "1", VarLabel: "RHobT", VarTag: "rh"},
		{Fhr: "3", VarLabel: "Temperature", VarTag: "temp"},
		{Fhr: "3", VarLabel: "Wind", VarTag: "wind"},
		{Fhr: "3", VarLabel: "RHobT", VarTag: "rh"},
		{Fhr: "6", VarLabel: "Temperature", VarTag: "temp"},
		{Fhr: "6", VarLabel: "Wind", VarTag: "wind"},
		{Fhr: "6", VarLabel: "RHobT", VarTag: "rh"},
		{Fhr: "12", VarLabel: "Temperature", VarTag: "temp"},
		{Fhr: "12", VarLabel: "Wind", VarTag: "wind"},
		{Fhr: "12", VarLabel: "RHobT", VarTag: "rh"},
	}

	totalSteps := len(amdarDieoffVars) + len(amdarProfConfigs)
	currentStep := 0

	// 1. AMDAR Dieoff
	for _, v := range amdarDieoffVars {
		currentStep++
		fn := fmt.Sprintf("amdar_dieoff_%s.png", v.Tag)
		outPath := filepath.Join(outDir, fn)

		if onProgress != nil {
			onProgress(currentStep, totalSteps, fmt.Sprintf("AMDAR Dieoff: %s", v.MATSLabel))
		}

		if forceFetch || !isFileValid(outPath) {
			time.Sleep(300 * time.Millisecond)
			if err := c.RetryFetch(fn, 2, 2*time.Second, func() error {
				return c.fetchSingleAMDARDieoff(validModels, uaDates, colorMap, v.MATSLabel, outPath)
			}); err != nil {
				c.Log("  [AMDAR WARNING] Failed to fetch AMDAR Dieoff (%s): %v. Slot left empty.\n", v.MATSLabel, err)
				_ = os.Remove(outPath)
			}
		} else {
			c.Log("  [CACHE HIT] Using cached: %s\n", fn)
		}
	}

	// 2. AMDAR Profiles
	for _, cfg := range amdarProfConfigs {
		currentStep++
		fn := fmt.Sprintf("amdar_prof_%sh_%s.png", cfg.Fhr, cfg.VarTag)
		outPath := filepath.Join(outDir, fn)

		if onProgress != nil {
			onProgress(currentStep, totalSteps, fmt.Sprintf("AMDAR Profile %sh: %s", cfg.Fhr, cfg.VarLabel))
		}

		if forceFetch || !isFileValid(outPath) {
			time.Sleep(300 * time.Millisecond)
			if err := c.RetryFetch(fn, 2, 2*time.Second, func() error {
				return c.fetchSingleAMDARProfile(validModels, uaDates, colorMap, cfg, outPath)
			}); err != nil {
				c.Log("  [AMDAR WARNING] Failed to fetch AMDAR Profile (%s): %v. Slot left empty.\n", fn, err)
				_ = os.Remove(outPath)
			}
		} else {
			c.Log("  [CACHE HIT] Using cached: %s\n", fn)
		}
	}

	return nil
}

func (c *Client) fetchSingleAMDARDieoff(
	models []Model,
	uaDates string,
	colorMap map[string]string,
	varLabel string,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/upperair"); err != nil {
		return fmt.Errorf("failed to open MATS UpperAir page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", "Dieoff")); err != nil {
		return fmt.Errorf("failed to select Dieoff: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	_ = chromedp.Do(tabCtx, chromedp.Evaluate[chromedp.Void](fmt.Sprintf("(() => { %s })()", SetParamJS("database", "AMDAR (Traditional)"))))
	time.Sleep(1000 * time.Millisecond)

	// RMSE
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("database", "AMDAR (Traditional)"),
			SetParamJS("data-source", m.ID),
			SetParamJS("region", "Full model domain"),
			SetParamJS("statistic", "RMSE"),
			SetParamJS("variable", varLabel),
			SetParamJS("dieoff-type", "Dieoff"),
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

	// Bias
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("database", "AMDAR (Traditional)"),
			SetParamJS("data-source", m.ID),
			SetParamJS("region", "Full model domain"),
			SetParamJS("statistic", "Bias (Model - Obs)"),
			SetParamJS("variable", varLabel),
			SetParamJS("dieoff-type", "Dieoff"),
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

func (c *Client) fetchSingleAMDARProfile(
	models []Model,
	uaDates string,
	colorMap map[string]string,
	cfg UAProfileConfig,
	outPath string,
) error {
	tabCtx, cancel := c.session.NewTab(75 * time.Second)
	defer cancel()

	if err := c.OpenMATSPage(tabCtx, "https://gsl.noaa.gov/mats/upperair"); err != nil {
		return fmt.Errorf("failed to open MATS UpperAir page: %w", err)
	}
	if err := chromedp.Do(tabCtx, chromedp.SetValue("#plotTypes-selector", "Profile")); err != nil {
		return fmt.Errorf("failed to select Profile: %w", err)
	}

	time.Sleep(600 * time.Millisecond)

	_ = chromedp.Do(tabCtx, chromedp.Evaluate[chromedp.Void](fmt.Sprintf("(() => { %s })()", SetParamJS("database", "AMDAR (Traditional)"))))
	time.Sleep(1000 * time.Millisecond)

	// RMSE
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("database", "AMDAR (Traditional)"),
			SetParamJS("data-source", m.ID),
			SetParamJS("region", "Full model domain"),
			SetParamJS("statistic", "RMSE"),
			SetParamJS("variable", cfg.VarLabel),
			SetParamJS("forecast-length", cfg.Fhr),
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

	// Bias
	for _, m := range models {
		js := strings.Join([]string{
			SetParamJS("database", "AMDAR (Traditional)"),
			SetParamJS("data-source", m.ID),
			SetParamJS("region", "Full model domain"),
			SetParamJS("statistic", "Bias (Model - Obs)"),
			SetParamJS("variable", cfg.VarLabel),
			SetParamJS("forecast-length", cfg.Fhr),
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
	return c.ExportStyledChart(tabCtx, outPath, colorMap, models, 900, 900)
}
