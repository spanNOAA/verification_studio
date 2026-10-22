package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"verification_studio/browser"
	"verification_studio/hpc"
	"verification_studio/mats"
	"verification_studio/mrms"
	"verification_studio/report"
)

// Request defines the options for a verification execution run.
type Request struct {
	Title        string          `json:"title"`
	OutputDir    string          `json:"output_dir"`
	ReportName   string          `json:"report_name"`
	ModelsText   string          `json:"models_text"`
	SfcDates     string          `json:"sfc_dates"`
	UaDates      string          `json:"ua_dates"`
	RadarDates   string          `json:"radar_dates"`
	CeilVisDates string          `json:"ceilvis_dates"`
	Regions      []string        `json:"regions"`
	SkipFetch    bool            `json:"skip_fetch"`
	ForceFetch   bool            `json:"force_fetch"`
	IncludeCref  bool            `json:"include_cref"`
	RadarCases   json.RawMessage `json:"radar_cases"`
}

// ExtractShortName derives a concise, readable experiment name from a MATS model ID.
func ExtractShortName(modelID string) string {
	mid := strings.TrimSpace(modelID)
	var name string
	if strings.Contains(mid, "_ret_") {
		parts := strings.SplitN(mid, "_ret_", 2)
		name = parts[1]
	} else if strings.Contains(mid, "_ret") {
		parts := strings.SplitN(mid, "_ret", 2)
		name = strings.TrimLeft(parts[1], "_")
	} else {
		parts := strings.Split(mid, "_")
		name = parts[len(parts)-1]
	}

	lower := strings.ToLower(name)
	if idx := strings.Index(lower, "retv"); idx >= 0 {
		name = name[idx:]
	}

	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	return re.ReplaceAllString(name, "_")
}

// ParseModelEntry parses a line from the Models input box.
func ParseModelEntry(entry string) mats.Model {
	entry = strings.TrimSpace(entry)
	var customName string

	if strings.Contains(entry, ":") && strings.Contains(entry, " - ") {
		partsDash := strings.SplitN(entry, " - ", 2)
		if strings.Contains(partsDash[0], ":") && !strings.Contains(strings.SplitN(partsDash[0], ":", 2)[0], "/") {
			pName := strings.SplitN(partsDash[0], ":", 2)
			customName = strings.TrimSpace(pName[0])
			entry = strings.TrimSpace(pName[1]) + " - " + strings.TrimSpace(partsDash[1])
		}
	} else if strings.Contains(entry, ":") && !strings.Contains(entry, " - ") {
		parts := strings.SplitN(entry, ":", 2)
		if !strings.Contains(parts[0], "/") {
			customName = strings.TrimSpace(parts[0])
			entry = strings.TrimSpace(parts[1])
		}
	}

	var modelID, cluster, path string
	if strings.Contains(entry, " - ") {
		parts := strings.SplitN(entry, " - ", 2)
		modelID = strings.TrimSpace(parts[0])
		pathSpec := strings.TrimSpace(parts[1])
		if strings.Contains(pathSpec, ":") && !strings.HasPrefix(pathSpec, "/") {
			subParts := strings.SplitN(pathSpec, ":", 2)
			cPart := strings.TrimSpace(subParts[0])
			if strings.Contains(cPart, "@") {
				atParts := strings.SplitN(cPart, "@", 2)
				cPart = atParts[1]
			}
			cluster = strings.ToLower(strings.TrimSpace(cPart))
			path = strings.TrimSpace(subParts[1])
		} else {
			path = pathSpec
		}
	} else {
		modelID = entry
	}

	cluster = hpc.NormalizeCluster(cluster, path)
	name := customName
	if name == "" {
		name = ExtractShortName(modelID)
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	name = re.ReplaceAllString(name, "_")

	return mats.Model{
		ID:      modelID,
		Name:    name,
		Cluster: cluster,
		Path:    path,
	}
}

// ParseModels parses all non-empty lines and assigns distinct high-contrast colors.
func ParseModels(modelsText string) []mats.Model {
	lines := strings.Split(modelsText, "\n")
	var models []mats.Model
	idx := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		m := ParseModelEntry(l)
		m.Color = mats.DefaultPalette[idx%len(mats.DefaultPalette)]
		models = append(models, m)
		idx++
	}
	return models
}

// ParseRadarCases decodes case studies from JSON array.
func ParseRadarCases(raw json.RawMessage) []hpc.RadarCase {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil
	}

	var cases []hpc.RadarCase
	idx := 1
	for _, item := range rawItems {
		// Try object format: {"date": "2024-05-08", "cycle": "00", "fhrs": [1, 3]}
		type caseObj struct {
			Date  any   `json:"date"`
			Cycle any   `json:"cycle"`
			Fhrs  []int `json:"fhrs"`
		}
		var co caseObj
		if err := json.Unmarshal(item, &co); err == nil && co.Date != nil {
			cdate := strings.ReplaceAll(fmt.Sprintf("%v", co.Date), "-", "")
			cycleInt, _ := strconv.Atoi(fmt.Sprintf("%v", co.Cycle))
			cycleStr := fmt.Sprintf("%02d", cycleInt)

			monDay := cdate
			if t, err := time.Parse("20060102", cdate); err == nil {
				monDay = fmt.Sprintf("%s %d", t.Format("Jan"), t.Day())
			}

			for _, fhr := range co.Fhrs {
				title := fmt.Sprintf("%d-h fcst from %sZ %s", fhr, cycleStr, monDay)
				cases = append(cases, hpc.RadarCase{
					Index: strconv.Itoa(idx),
					Date:  cdate,
					Cycle: cycleStr,
					Fhr:   fhr,
					Title: title,
				})
				idx++
			}
			continue
		}

		// Try array format: [orig_idx, cdate, cycle, fhr, case_title]
		var arr []any
		if err := json.Unmarshal(item, &arr); err == nil && len(arr) >= 5 {
			fhr, _ := strconv.Atoi(fmt.Sprintf("%v", arr[3]))
			cases = append(cases, hpc.RadarCase{
				Index: fmt.Sprintf("%v", arr[0]),
				Date:  fmt.Sprintf("%v", arr[1]),
				Cycle: fmt.Sprintf("%v", arr[2]),
				Fhr:   fhr,
				Title: fmt.Sprintf("%v", arr[4]),
			})
			idx++
		}
	}

	return cases
}

// Run executes the complete verification workflow in pure Go.
func Run(ctx context.Context, req Request, logFn func(string)) error {
	startTime := time.Now()
	defer func() {
		// Automatically disconnect and terminate background HPC session daemons
		hpc.DisconnectAll()
	}()

	log := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if logFn != nil {
			logFn(msg)
		}
		fmt.Print(msg)
	}

	radarDates := strings.TrimSpace(req.RadarDates)
	if radarDates == "" {
		radarDates = req.SfcDates
	}
	ceilvisDates := strings.TrimSpace(req.CeilVisDates)
	if ceilvisDates == "" {
		ceilvisDates = req.SfcDates
	}

	log("\n==================================================================\n")
	log(" Initializing Verification Pipeline...\n")
	log(" Title   : %s\n", req.Title)
	log(" Output  : %s\n", req.OutputDir)
	log(" Report  : %s (.pptx / .html)\n", req.ReportName)
	log(" Periods : Surface [%s] | Upper-Air [%s] | Radar [%s] | Ceil/Vis [%s]\n", req.SfcDates, req.UaDates, radarDates, ceilvisDates)
	log("==================================================================\n\n")

	models := ParseModels(req.ModelsText)
	if len(models) == 0 {
		return fmt.Errorf("no valid models specified in configuration")
	}

	colorMap := make(map[string]string)
	log("Configured Evaluated Experiments (%d):\n", len(models))
	for _, m := range models {
		colorMap[m.ID] = m.Color
		colorMap[m.Name] = m.Color
		remoteInfo := ""
		if m.Path != "" {
			remoteInfo = fmt.Sprintf(" -> %s:%s", m.Cluster, m.Path)
		}
		log("  • %s (%s) [%s]%s\n", m.Name, m.ID, m.Color, remoteInfo)
	}
	log("\n")

	// Setup directories
	matsDir := filepath.Join(req.OutputDir, "mats")
	crefDir := filepath.Join(req.OutputDir, "cref")
	if req.ForceFetch && !req.SkipFetch {
		log("[FORCE REFRESH] 🔄 Force refresh enabled: invalidating all cached charts to ensure clean, fresh evaluation...\n")
		_ = os.RemoveAll(matsDir)
		if req.IncludeCref {
			_ = os.RemoveAll(crefDir)
		}
	}
	if err := os.MkdirAll(matsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create mats directory: %w", err)
	}
	if req.IncludeCref {
		if err := os.MkdirAll(crefDir, 0o755); err != nil {
			return fmt.Errorf("failed to create cref directory: %w", err)
		}
	}

	regions := req.Regions
	if len(regions) == 0 {
		regions = []string{"all"}
	}

	radarCases := ParseRadarCases(req.RadarCases)
	hasRadarCases := req.IncludeCref && len(radarCases) > 0

	// 1. Launch Headless Browser Session
	log(">>> Initializing host Chromium browser session...\n")
	session, err := browser.NewSession(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to launch headless browser: %w", err)
	}
	defer session.Close()
	log("  Browser running: %s\n\n", session.Path())

	matsClient := mats.NewClient(session)
	matsClient.SetLogger(func(s string) { log("%s", s) })
	mrmsClient := mrms.NewClient(session)
	mrmsClient.SetLogger(func(s string) { log("%s", s) })

	// Calculate dynamic stage weights
	// Standard stages:
	// 1. Surface Dieoff
	// 2. Surface Time Series
	// 3. Upper Air RAOBs Dieoff
	// 4. Upper Air RAOBs Profiles
	// 5. Upper Air AMDAR
	// 6. Radar Reflectivity
	// 7. Cloud Ceiling
	// 8. Visibility
	// + 9. HPC Remote Images (if cases)
	// + 10. MRMS Radar Observations (if cases)
	// + 11. Presentation Reports (PPTX + HTML)

	totalStages := 9
	if hasRadarCases {
		totalStages += 2
	}

	reportProgress := func(stageNum int, stagePct int, stageTitle string) {
		overallPct := int((float64(stageNum-1)/float64(totalStages))*100.0 + (float64(stagePct)/100.0)*(100.0/float64(totalStages)))
		if overallPct > 100 {
			overallPct = 100
		}
		log("[PROGRESS: %d%%, stage=%d%%] [%s]\n", overallPct, stagePct, stageTitle)
	}

	stageIdx := 0

	if !req.SkipFetch {
		isolateBrowser := func(stageName string) {
			if rErr := session.Restart(ctx); rErr != nil {
				log("  [BROWSER REFRESH WARNING] Failed to isolate session for %s: %v\n", stageName, rErr)
			}
		}

		// Stage 1 & 2 (Optional, early): Remote HPC Radar Images & MRMS Observations
		// Running HPC retrieval first guarantees user can provide PIN/YubiKey MFA immediately at pipeline launch
		if hasRadarCases {
			stageIdx++
			log(">>> [%d/%d] Retrieving Synoptic Radar Forecast Images from HPC Clusters...\n", stageIdx, totalStages)
			reportProgress(stageIdx, 0, "HPC Radar Retrieval")

			allCached, totalImg := false, 0
			if !req.ForceFetch {
				allCached, totalImg = hpc.AllRadarImagesCached(models, crefDir, radarCases)
			}

			if allCached && totalImg > 0 {
				log("  [ALL CACHED] All radar forecast images (%d files) already available locally. Skipping HPC authentication & retrieval.\n", totalImg)
				reportProgress(stageIdx, 100, "All cached locally")
			} else {
				err = hpc.DownloadRadarImages(models, crefDir, radarCases, req.ForceFetch, func(pct int, msg string) {
					reportProgress(stageIdx, pct, msg)
				}, func(s string) { log("%s", s) })
				if err != nil {
					log("  [HPC RADAR ERROR] %v\n", err)
				}
			}

			isolateBrowser("MRMS Observations")
			stageIdx++
			log("\n>>> [%d/%d] Retrieving Observed MRMS Maps from NOAA NSSL Product Viewer...\n", stageIdx, totalStages)
			reportProgress(stageIdx, 0, "MRMS Observations")

			err = mrmsClient.FetchRadarImages(radarCases, crefDir, req.ForceFetch, func(pct int, msg string) {
				reportProgress(stageIdx, pct, msg)
			})
			if err != nil {
				log("  [MRMS RADAR ERROR] %v\n", err)
			}
		}

		// Surface Dieoff
		isolateBrowser("Surface Dieoff")
		stageIdx++
		log("\n>>> [%d/%d] Fetching Surface Dieoff Verification...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Surface Dieoff")
		err = matsClient.FetchSurfaceDieoff(ctx, models, req.SfcDates, colorMap, matsDir, regions, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [SURFACE ERROR] %v\n", err)
		}

		// Surface Time Series
		isolateBrowser("Surface Time Series")
		stageIdx++
		log("\n>>> [%d/%d] Fetching Surface Time Series (1h, 3h, 12h, 24h)...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Surface Time Series")
		err = matsClient.FetchSurfaceTimeSeries(ctx, models, req.SfcDates, colorMap, matsDir, regions, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [SURFACE TS ERROR] %v\n", err)
		}

		// Upper Air Dieoff against RAOBs
		isolateBrowser("Upper Air Dieoff")
		stageIdx++
		log("\n>>> [%d/%d] Fetching Upper Air Dieoff against RAOBs...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Upper Air RAOBs Dieoff")
		err = matsClient.FetchUpperAirDieoff(ctx, models, req.UaDates, colorMap, matsDir, regions, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [UPPER AIR ERROR] %v\n", err)
		}

		// Upper Air Profiles
		isolateBrowser("Upper Air Profiles")
		stageIdx++
		log("\n>>> [%d/%d] Fetching Upper Air Profiles (1h, 3h, 6h, 12h)...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Upper Air Profiles")
		err = matsClient.FetchUpperAirProfiles(ctx, models, req.UaDates, colorMap, matsDir, regions, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [UPPER AIR PROFILE ERROR] %v\n", err)
		}

		// Upper Air AMDAR
		isolateBrowser("Upper Air AMDAR")
		stageIdx++
		log("\n>>> [%d/%d] Checking Upper Air AMDAR Verification...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Upper Air AMDAR")
		err = matsClient.FetchUpperAirAMDAR(ctx, models, req.UaDates, colorMap, matsDir, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [AMDAR ERROR] %v\n", err)
		}

		// Radar Composite Reflectivity
		isolateBrowser("Radar Reflectivity")
		stageIdx++
		log("\n>>> [%d/%d] Fetching Radar Composite Reflectivity (15, 25, 35 dBZ)...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Radar Reflectivity")
		err = matsClient.FetchReflectivity(ctx, models, radarDates, colorMap, matsDir, regions, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [RADAR ERROR] %v\n", err)
		}

		// Cloud Ceiling
		isolateBrowser("Cloud Ceiling")
		stageIdx++
		log("\n>>> [%d/%d] Fetching Cloud Ceiling Verification (500ft, 1000ft, 3000ft)...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Cloud Ceiling")
		err = matsClient.FetchCeiling(ctx, models, ceilvisDates, colorMap, matsDir, regions, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [CEILING ERROR] %v\n", err)
		}

		// Visibility
		isolateBrowser("Visibility")
		stageIdx++
		log("\n>>> [%d/%d] Fetching Surface Visibility Verification (1mi, 3mi, 5mi)...\n", stageIdx, totalStages)
		reportProgress(stageIdx, 0, "Visibility")
		err = matsClient.FetchVisibility(ctx, models, ceilvisDates, colorMap, matsDir, regions, req.ForceFetch, func(curr, total int, msg string) {
			pct := int(float64(curr) / float64(total) * 100.0)
			reportProgress(stageIdx, pct, msg)
		})
		if err != nil {
			log("  [VISIBILITY ERROR] %v\n", err)
		}
	} else {
		log(">>> [--skip-fetch] Skipping online data downloads; reusing cached local plots.\n")
	}

	// Final Stage: Presentation Generation (PPTX + HTML)
	stageIdx++
	log("\n>>> [%d/%d] Compiling Synchronized PowerPoint & HTML Presentations...\n", stageIdx, totalStages)
	reportProgress(stageIdx, 20, "Compiling Presentations")

	pptxPath := filepath.Join(req.OutputDir, req.ReportName+".pptx")
	htmlPath := filepath.Join(req.OutputDir, req.ReportName+".html")

	reportCfg := report.Config{
		Title: req.Title,
		Dates: report.DatesConfig{
			Surface:  req.SfcDates,
			UpperAir: req.UaDates,
			Radar:    radarDates,
			CeilVis:  ceilvisDates,
		},
		Models:     models,
		Regions:    regions,
		CrefDir:    crefDir,
		SkipCref:   !req.IncludeCref,
		RadarCases: radarCases,
	}

	slidesData, err := report.BuildPresentation(reportCfg, matsDir, pptxPath)
	if err != nil {
		return fmt.Errorf("failed to build PowerPoint presentation: %w", err)
	}

	reportProgress(stageIdx, 70, "Compiling HTML Report")
	if err := report.BuildHTMLReport(reportCfg, matsDir, htmlPath, slidesData); err != nil {
		return fmt.Errorf("failed to build HTML presentation: %w", err)
	}

	reportProgress(stageIdx, 100, "Generation Complete")

	elapsed := time.Since(startTime)
	log("\n==================================================================\n")
	log(" [SUCCESS] Verification pipeline completed in %s!\n", elapsed.Round(time.Second))
	log("   • PowerPoint Presentation: %s\n", pptxPath)
	log("   • Interactive Web Report : %s\n", htmlPath)
	log("==================================================================\n")

	return nil
}
