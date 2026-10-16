package report

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// BuildHTMLReport generates an identical, interactive HTML report with exact 1-to-1 slide alignment to the PPT presentation.
func BuildHTMLReport(cfg Config, matsDir, outputHTML string, slidesData []SlideData) error {
	fmt.Printf("\n>>> Compiling HTML Presentation Report: %s...\n", outputHTML)
	htmlDir := filepath.Dir(outputHTML)
	if err := os.MkdirAll(htmlDir, 0o755); err != nil {
		return err
	}

	relImg := func(path string) string {
		if path == "" {
			return ""
		}
		rel, err := filepath.Rel(htmlDir, path)
		if err != nil {
			return path
		}
		return rel
	}

	titleText := cfg.Title
	if titleText == "" {
		titleText = "Numerical Weather Prediction Verification Report"
	}

	var slidesHTML []string
	var navOptions []string

	svgSolid := `<svg width="24" height="6" style="vertical-align: middle;"><line x1="0" y1="3" x2="24" y2="3" stroke="#334155" stroke-width="3" /></svg>`
	svgDash := `<svg width="24" height="6" style="vertical-align: middle;"><line x1="0" y1="3" x2="24" y2="3" stroke="#334155" stroke-width="3" stroke-dasharray="5,3" /></svg>`

	for idx, slide := range slidesData {
		slideNum := idx + 1
		stype := slide.Type
		stitle := slide.Title
		if stitle == "" {
			stitle = fmt.Sprintf("Slide %d", slideNum)
		}
		safeTitle := html.EscapeString(stitle)
		navOptions = append(navOptions, fmt.Sprintf(`<option value="%d">%d. %s</option>`, slideNum, slideNum, safeTitle))

		slideContent := ""

		switch stype {
		case "title":
			var bulletsHTML []string
			for _, b := range slide.Bullets {
				cleanText := strings.TrimSpace(strings.TrimLeft(b.Text, "•"))
				sText := html.EscapeString(cleanText)
				if b.IsBold {
					bulletsHTML = append(bulletsHTML, fmt.Sprintf(`<li class="bullet-header"><strong>%s</strong></li>`, sText))
				} else {
					bulletsHTML = append(bulletsHTML, fmt.Sprintf(`<li class="bullet-item">%s</li>`, sText))
				}
			}

			var modelChips []string
			for _, m := range slide.Models {
				color := m.Color
				if color == "" {
					color = "#2563eb"
				}
				modelChips = append(modelChips, fmt.Sprintf(
					`<span class="model-badge" style="border-left: 5px solid %s"><strong>%s</strong> <small>(%s)</small></span>`,
					color, html.EscapeString(m.Name), html.EscapeString(m.ID),
				))
			}

			sfcD := html.EscapeString(slide.Dates.Surface)
			if sfcD == "" {
				sfcD = "N/A"
			}
			uaD := html.EscapeString(slide.Dates.UpperAir)
			if uaD == "" {
				uaD = "N/A"
			}

			slideContent = fmt.Sprintf(`
			<div class="title-slide-container">
			  <div class="title-hero">
			    <span class="report-tag">NOAA GSL · Verification Executive Briefing</span>
			    <h1 class="main-title">%s</h1>
			  </div>
			  <div class="title-body">
			    <div class="models-panel">
			      <h3>Evaluated Experiments:</h3>
			      <div class="models-grid">%s</div>
			      <div class="date-ranges">
			        <div><strong>Surface Period:</strong> %s</div>
			        <div><strong>Upper-Air Period:</strong> %s</div>
			      </div>
			    </div>
			    <div class="sources-panel">
			      <h3>Verification Framework:</h3>
			      <ul class="bullet-list">%s</ul>
			    </div>
			  </div>
			</div>
			`, safeTitle, strings.Join(modelChips, ""), sfcD, uaD, strings.Join(bulletsHTML, ""))

		case "3panel":
			img1 := relImg(slide.Img1)
			img2 := relImg(slide.Img2)
			img3 := relImg(slide.Img3)
			lbl1 := html.EscapeString(slide.Lbl1)
			lbl2 := html.EscapeString(slide.Lbl2)
			lbl3 := html.EscapeString(slide.Lbl3)
			metric := html.EscapeString(slide.Metric)
			if metric == "" {
				metric = "RMSE"
			}

			var expLines []string
			for _, m := range slide.Models {
				c := m.Color
				if c == "" {
					c = "#64748b"
				}
				expLines = append(expLines, fmt.Sprintf(
					`<div class="legend-row"><span class="color-dot" style="background:%s"></span><span>%s</span></div>`,
					c, html.EscapeString(m.Name),
				))
			}

			styleLegend := ""
			if slide.IsBias {
				styleLegend = fmt.Sprintf(`<div class="legend-subtitle">Metric:</div><div class="legend-row">%s<span>Dash: BIAS</span></div>`, svgDash)
			} else if metric == "COMBINED" {
				styleLegend = fmt.Sprintf(`<div class="legend-subtitle">Line Styles:</div><div class="legend-row">%s<span>Solid: RMSE</span></div><div class="legend-row">%s<span>Dash: BIAS</span></div>`, svgSolid, svgDash)
			} else {
				styleLegend = fmt.Sprintf(`<div class="legend-subtitle">Metric:</div><div class="legend-row">%s<span>Solid: %s</span></div>`, svgSolid, metric)
			}

			p1HTML := `<div class="no-img">No Image</div>`
			if img1 != "" {
				p1HTML = fmt.Sprintf(`<img src="%s" alt="%s" onclick="openLightbox(this)">`, img1, lbl1)
			}
			p2HTML := `<div class="no-img">No Image</div>`
			if img2 != "" {
				p2HTML = fmt.Sprintf(`<img src="%s" alt="%s" onclick="openLightbox(this)">`, img2, lbl2)
			}
			p3HTML := `<div class="no-img">No Image</div>`
			if img3 != "" {
				p3HTML = fmt.Sprintf(`<img src="%s" alt="%s" onclick="openLightbox(this)">`, img3, lbl3)
			}

			slideContent = fmt.Sprintf(`
			<div class="slide-header"><h2>%s</h2></div>
			<div class="grid-3panel">
			  <div class="panel-box p1">
			    <div class="panel-label">%s</div>
			    %s
			  </div>
			  <div class="panel-box p2">
			    <div class="panel-label">%s</div>
			    %s
			  </div>
			  <div class="panel-box p3">
			    <div class="panel-label">%s</div>
			    %s
			  </div>
			  <div class="panel-box legend-box">
			    <div class="legend-title">Experiments:</div>
			    %s
			    %s
			  </div>
			</div>
			`, safeTitle, lbl1, p1HTML, lbl2, p2HTML, lbl3, p3HTML, strings.Join(expLines, ""), styleLegend)

		case "integrated_ts":
			col1Name := html.EscapeString(slide.Col1Name)
			col2Name := html.EscapeString(slide.Col2Name)
			var rowHTML []string
			for _, r := range slide.Rows {
				rlbl := html.EscapeString(r.Label)
				rImg1 := relImg(r.Img1)
				rImg2 := relImg(r.Img2)

				img1Box := `<div class="no-img">No Image</div>`
				if rImg1 != "" {
					img1Box = fmt.Sprintf(`<img src="%s" alt="%s %s" onclick="openLightbox(this)">`, rImg1, rlbl, col1Name)
				}
				img2Box := `<div class="no-img">No Image</div>`
				if rImg2 != "" {
					img2Box = fmt.Sprintf(`<img src="%s" alt="%s %s" onclick="openLightbox(this)">`, rImg2, rlbl, col2Name)
				}

				rowHTML = append(rowHTML, fmt.Sprintf(`
				<div class="ts-row">
				  <div class="ts-row-label">%s</div>
				  <div class="ts-img-box">%s</div>
				  <div class="ts-img-box">%s</div>
				</div>
				`, rlbl, img1Box, img2Box))
			}

			var expLines []string
			for _, m := range slide.Models {
				c := m.Color
				if c == "" {
					c = "#64748b"
				}
				expLines = append(expLines, fmt.Sprintf(
					`<div class="legend-row"><span class="color-dot" style="background:%s"></span><span>%s</span></div>`,
					c, html.EscapeString(m.Name),
				))
			}

			slideContent = fmt.Sprintf(`
			<div class="slide-header"><h2>%s</h2></div>
			<div class="ts-integrated-container">
			  <div class="ts-main-grid">
			    <div class="ts-header-row">
			      <div class="ts-label-space"></div>
			      <div class="ts-col-header">%s</div>
			      <div class="ts-col-header">%s</div>
			    </div>
			    %s
			  </div>
			  <div class="ts-legend-sidebar">
			    <div class="legend-title">Experiments:</div>
			    %s
			    <div class="legend-subtitle">Columns:</div>
			    <div class="legend-row"><span>• Left: %s</span></div>
			    <div class="legend-row"><span>• Right: %s</span></div>
			    <div class="legend-subtitle">Line Styles:</div>
			    <div class="legend-row">%s<span>Solid: %s</span></div>
			    <div class="legend-row">%s<span>Dash: %s</span></div>
			  </div>
			</div>
			`, safeTitle, col1Name, col2Name, strings.Join(rowHTML, ""), strings.Join(expLines, ""), col1Name, col2Name, svgSolid, col1Name, svgDash, col2Name)

		case "profile":
			imgT := relImg(slide.ImgT)
			imgW := relImg(slide.ImgW)
			imgRH := relImg(slide.ImgRH)

			var expLines []string
			for _, m := range slide.Models {
				c := m.Color
				if c == "" {
					c = "#64748b"
				}
				expLines = append(expLines, fmt.Sprintf(
					`<div class="legend-row"><span class="color-dot" style="background:%s"></span><span>%s</span></div>`,
					c, html.EscapeString(m.Name),
				))
			}

			tBox := `<div class="no-img">No Image</div>`
			if imgT != "" {
				tBox = fmt.Sprintf(`<img src="%s" alt="Temperature Profile" onclick="openLightbox(this)">`, imgT)
			}
			wBox := `<div class="no-img">No Image</div>`
			if imgW != "" {
				wBox = fmt.Sprintf(`<img src="%s" alt="Wind Profile" onclick="openLightbox(this)">`, imgW)
			}
			rhBox := `<div class="no-img">No Image</div>`
			if imgRH != "" {
				rhBox = fmt.Sprintf(`<img src="%s" alt="RH Profile" onclick="openLightbox(this)">`, imgRH)
			}

			if imgRH != "" {
				slideContent = fmt.Sprintf(`
				<div class="slide-header"><h2>%s</h2></div>
				<div class="grid-profile">
				  <div class="profile-box p-temp">
				    <div class="profile-label">Temperature (t)</div>
				    %s
				  </div>
				  <div class="profile-box p-wind">
				    <div class="profile-label">Wind</div>
				    %s
				  </div>
				  <div class="profile-box p-rh">
				    <div class="profile-label">Relative Humidity (RHobT)</div>
				    %s
				  </div>
				  <div class="profile-box legend-box">
				    <div class="legend-title">Experiments:</div>
				    %s
				    <div class="legend-subtitle">Line Styles:</div>
				    <div class="legend-row">%s<span>Solid: RMSE</span></div>
				    <div class="legend-row">%s<span>Dash: BIAS</span></div>
				  </div>
				</div>
				`, safeTitle, tBox, wBox, rhBox, strings.Join(expLines, ""), svgSolid, svgDash)
			} else {
				slideContent = fmt.Sprintf(`
				<div class="slide-header"><h2>%s</h2></div>
				<div class="grid-profile-amdar">
				  <div class="profile-box p-temp">
				    <div class="profile-label">Temperature (t)</div>
				    %s
				  </div>
				  <div class="profile-box p-wind">
				    <div class="profile-label">Wind</div>
				    %s
				  </div>
				  <div class="profile-box legend-box profile-amdar-legend">
				    <div class="legend-title">Experiments:</div>
				    %s
				    <div class="legend-subtitle">Line Styles:</div>
				    <div class="legend-row">%s<span>Solid: RMSE</span></div>
				    <div class="legend-row">%s<span>Dash: BIAS</span></div>
				  </div>
				</div>
				`, safeTitle, tBox, wBox, strings.Join(expLines, ""), svgSolid, svgDash)
			}

		case "section_header":
			regSub := html.EscapeString(slide.Subtitle)
			slideContent = fmt.Sprintf(`
			<div class="section-divider-slide">
			  <div class="section-divider-inner">
			    <span class="section-pill">REGIONAL VERIFICATION CHAPTER</span>
			    <h1 class="section-hero-title">%s</h1>
			    <p class="section-hero-sub">%s</p>
			  </div>
			</div>
			`, safeTitle, regSub)

		case "radar_case":
			caseT := html.EscapeString(slide.CaseTitle)
			var cardBoxes []string
			for _, card := range slide.Cards {
				relP := relImg(card.ImgPath)
				safeName := html.EscapeString(card.Name)
				imgTag := `<div class="no-img">No Image</div>`
				if relP != "" {
					imgTag = fmt.Sprintf(`<img src="%s" alt="%s" onclick="openLightbox(this)">`, relP, safeName)
				}
				cardBoxes = append(cardBoxes, fmt.Sprintf(`
				<div class="case-box">
				  <div class="case-label">%s</div>
				  %s
				</div>
				`, safeName, imgTag))
			}

			slideContent = fmt.Sprintf(`
			<div class="slide-header"><h2>Composite Reflectivity (dBZ) Comparison: %s</h2></div>
			<div class="radar-case-grid">
			  %s
			</div>
			`, caseT, strings.Join(cardBoxes, ""))
		}

		sReg := slide.Region
		if slide.Type == "radar_case" {
			sReg = "cases"
		} else if sReg == "" {
			sReg = "all"
		}

		activeClass := ""
		if slideNum == 1 {
			activeClass = " active"
		}

		slidesHTML = append(slidesHTML, fmt.Sprintf(`
		<section class="slide-card%s" id="slide-%d" data-slide-num="%d" data-region="%s">
		  <div class="slide-watermark">Slide %d / %d</div>
		  %s
		</section>
		`, activeClass, slideNum, slideNum, sReg, slideNum, len(slidesData), slideContent))
	}

	regions := cfg.Regions
	var tabsHTML []string
	hasCases := false
	for _, s := range slidesData {
		if s.Type == "radar_case" {
			hasCases = true
			break
		}
	}

	if len(regions) > 1 || hasCases {
		tabsHTML = append(tabsHTML, `<div class="region-tabs" id="regionTabs">`)
		tabsHTML = append(tabsHTML, `<button class="region-tab active" data-filter="all" onclick="filterRegion('all', this)">🌐 Full Domain</button>`)
		for _, r := range regions {
			if r == "east" {
				tabsHTML = append(tabsHTML, `<button class="region-tab" data-filter="east" onclick="filterRegion('east', this)">🗺️ Eastern Region</button>`)
			} else if r == "west" {
				tabsHTML = append(tabsHTML, `<button class="region-tab" data-filter="west" onclick="filterRegion('west', this)">🏔️ Western Region</button>`)
			}
		}
		if hasCases {
			tabsHTML = append(tabsHTML, `<button class="region-tab" data-filter="cases" onclick="filterRegion('cases', this)">📡 Radar Cases</button>`)
		}
		tabsHTML = append(tabsHTML, `</div>`)
	}

	renderedHTML := strings.ReplaceAll(htmlTemplate, "__TITLE__", html.EscapeString(titleText))
	renderedHTML = strings.ReplaceAll(renderedHTML, "__REGION_TABS__", strings.Join(tabsHTML, ""))
	renderedHTML = strings.ReplaceAll(renderedHTML, "__NAV_OPTIONS__", strings.Join(navOptions, "\n"))
	renderedHTML = strings.ReplaceAll(renderedHTML, "__TOTAL_SLIDES__", fmt.Sprintf("%d", len(slidesData)))
	renderedHTML = strings.ReplaceAll(renderedHTML, "__SLIDES_HTML__", strings.Join(slidesHTML, "\n"))

	tmpFile := fmt.Sprintf("%s.tmp.%d", outputHTML, os.Getpid())
	if err := os.WriteFile(tmpFile, []byte(renderedHTML), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, outputHTML); err != nil {
		return err
	}

	fmt.Printf("[DONE] Successfully generated interactive HTML presentation: %s\n", outputHTML)
	return nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>__TITLE__</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background: #0f172a;
      color: #0f172a;
      line-height: 1.4;
      overflow-x: hidden;
    }
    
    /* Top Toolbar */
    .toolbar {
      position: fixed;
      top: 0; left: 0; right: 0;
      height: 56px;
      background: rgba(16, 44, 87, 0.95);
      backdrop-filter: blur(8px);
      color: #ffffff;
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 16px;
      gap: 12px;
      z-index: 1000;
      box-shadow: 0 2px 10px rgba(0,0,0,0.3);
      overflow-x: auto;
    }
    .toolbar::-webkit-scrollbar { display: none; }
    .toolbar-left {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 0;
      flex-shrink: 1;
    }
    .toolbar-title {
      font-size: 14.5px;
      font-weight: 700;
      color: #bae6fd;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      flex-shrink: 1;
    }
    
    /* Region Navigation Tabs */
    .region-tabs {
      display: flex;
      align-items: center;
      gap: 4px;
      background: rgba(15, 23, 42, 0.6);
      padding: 3px 6px;
      border-radius: 8px;
      border: 1px solid #334155;
      flex-shrink: 0;
    }
    .region-tab {
      background: transparent;
      border: none;
      color: #94a3b8;
      height: 30px;
      padding: 0 10px;
      border-radius: 5px;
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: all 0.15s ease;
      white-space: nowrap;
      user-select: none;
    }
    .region-tab:hover {
      color: #f8fafc;
      background: rgba(255, 255, 255, 0.08);
    }
    .region-tab.active {
      background: #0284c7;
      color: #ffffff;
      box-shadow: 0 1px 3px rgba(0,0,0,0.2);
    }
    .slide-select {
      background: #1e293b;
      color: #f8fafc;
      border: 1px solid #334155;
      height: 34px;
      padding: 0 10px;
      border-radius: 6px;
      font-size: 12.5px;
      font-weight: 500;
      cursor: pointer;
      outline: none;
      max-width: 260px;
      text-overflow: ellipsis;
      flex-shrink: 1;
      box-sizing: border-box;
    }
    .slide-select:focus { border-color: #0284c7; }
    .toolbar-center {
      display: flex;
      align-items: center;
      gap: 8px;
      font-weight: 600;
      font-size: 13px;
      flex-shrink: 0;
    }
    .counter-badge {
      background: #0284c7;
      color: #ffffff;
      height: 34px;
      padding: 0 12px;
      border-radius: 6px;
      font-size: 12.5px;
      font-weight: 600;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      white-space: nowrap;
      flex-shrink: 0;
      box-sizing: border-box;
    }
    .toolbar-right {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-shrink: 0;
    }
    .tool-btn {
      background: #1e293b;
      color: #e2e8f0;
      border: 1px solid #475569;
      height: 34px;
      padding: 0 12px;
      border-radius: 6px;
      font-size: 12.5px;
      font-weight: 600;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      white-space: nowrap;
      flex-shrink: 0;
      line-height: 1;
      box-sizing: border-box;
      transition: all 0.15s ease;
      user-select: none;
    }
    .tool-btn:hover { background: #334155; color: #ffffff; border-color: #64748b; }
    .tool-btn:focus-visible { outline: 2px solid #38bdf8; outline-offset: 1px; }
    .tool-btn.active {
      background: #0284c7;
      border-color: #38bdf8;
      color: #ffffff;
      box-shadow: 0 1px 3px rgba(0,0,0,0.25);
    }
    .tool-btn.primary { background: #16a34a; border-color: #15803d; color: #fff; }
    .tool-btn.primary:hover { background: #15803d; border-color: #166534; }

    /* Main Container */
    .presentation-container {
      margin-top: 56px;
      padding: 24px 16px;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 28px;
    }

    /* 16:9 Slide Card */
    .slide-card {
      width: 100%;
      max-width: 1360px;
      aspect-ratio: 16 / 9;
      background: #ffffff;
      border-radius: 10px;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3), 0 8px 10px -6px rgba(0, 0, 0, 0.2);
      padding: 16px 22px 20px 22px;
      display: flex;
      flex-direction: column;
      position: relative;
      overflow: hidden;
      border: 1px solid #cbd5e1;
      scroll-margin-top: 68px;
    }
    
    .slide-watermark {
      position: absolute;
      bottom: 8px;
      right: 14px;
      font-size: 11px;
      font-weight: 700;
      color: #94a3b8;
      letter-spacing: 0.5px;
      pointer-events: none;
    }

    /* Slideshow mode styles */
    body.mode-slideshow .presentation-container {
      height: calc(100vh - 56px);
      padding: 12px;
      justify-content: center;
      overflow: hidden;
    }
    body.mode-slideshow .slide-card {
      display: none;
      height: calc(100vh - 84px);
      max-height: 85vh;
      width: auto;
    }
    body.mode-slideshow .slide-card.active {
      display: flex;
      animation: fadeIn 0.2s ease-in-out;
    }
    @keyframes fadeIn { from { opacity: 0; transform: scale(0.99); } to { opacity: 1; transform: scale(1); } }

    /* Regional Section Divider Slide */
    .section-divider-slide {
      height: 100%;
      background: linear-gradient(135deg, #102c57 0%, #0d1b3e 100%);
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 40px;
      color: #ffffff;
      text-align: center;
      box-shadow: inset 0 0 40px rgba(0,0,0,0.4);
    }
    .section-divider-inner {
      max-width: 800px;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 16px;
    }
    .section-pill {
      background: rgba(56, 189, 248, 0.15);
      color: #38bdf8;
      border: 1px solid rgba(56, 189, 248, 0.4);
      padding: 5px 14px;
      border-radius: 20px;
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 1.5px;
      text-transform: uppercase;
    }
    .section-hero-title {
      font-size: 32px;
      font-weight: 800;
      line-height: 1.25;
      color: #ffffff;
    }
    .section-hero-sub {
      font-size: 15px;
      color: #94a3b8;
      line-height: 1.6;
    }

    /* Common Slide Components */
    .slide-header {
      margin-bottom: 8px;
      text-align: center;
      flex-shrink: 0;
    }
    .slide-header h2 {
      font-size: 18px;
      font-weight: 800;
      color: #102c57;
      letter-spacing: -0.2px;
    }

    /* Title Slide */
    .title-slide-container {
      height: 100%;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      padding: 20px 24px;
    }
    .title-hero {
      border-bottom: 2px solid #e2e8f0;
      padding-bottom: 14px;
    }
    .report-tag {
      font-size: 12px;
      font-weight: 700;
      color: #0284c7;
      text-transform: uppercase;
      letter-spacing: 1px;
    }
    .main-title {
      font-size: 26px;
      font-weight: 800;
      color: #102c57;
      margin-top: 4px;
    }
    .title-body {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 28px;
      margin-top: 14px;
      flex: 1;
    }
    .models-panel, .sources-panel {
      background: #f8fafc;
      padding: 16px 20px;
      border-radius: 8px;
      border: 1px solid #e2e8f0;
      display: flex;
      flex-direction: column;
    }
    .models-panel h3, .sources-panel h3 {
      font-size: 14px;
      font-weight: 700;
      color: #334155;
      margin-bottom: 12px;
      border-bottom: 1px solid #cbd5e1;
      padding-bottom: 6px;
    }
    .models-grid {
      display: flex;
      flex-direction: column;
      gap: 8px;
      margin-bottom: 16px;
    }
    .model-badge {
      background: #ffffff;
      padding: 8px 12px;
      border-radius: 6px;
      border: 1px solid #cbd5e1;
      font-size: 13px;
      box-shadow: 0 1px 2px rgba(0,0,0,0.05);
    }
    .date-ranges {
      margin-top: auto;
      font-size: 12px;
      color: #475569;
      background: #e2e8f0;
      padding: 8px 12px;
      border-radius: 6px;
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .bullet-list {
      list-style: none;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .bullet-header {
      font-size: 13.5px;
      color: #1e3a8a;
      margin-top: 4px;
    }
    .bullet-item {
      font-size: 12.5px;
      color: #475569;
      padding-left: 14px;
      position: relative;
    }
    .bullet-item::before {
      content: "•";
      position: absolute;
      left: 0;
      color: #0284c7;
      font-weight: bold;
    }

    /* 3-Panel Slide Layout */
    .grid-3panel {
      display: grid;
      grid-template-columns: 1fr 1fr;
      grid-template-rows: 1fr 1fr;
      gap: 10px;
      flex: 1;
      position: relative;
    }
    .panel-box {
      position: relative;
      background: #f8fafc;
      border: 1px solid #e2e8f0;
      border-radius: 6px;
      overflow: hidden;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .panel-box img { max-width: 100%; max-height: 100%; object-fit: contain; cursor: zoom-in; }
    .panel-label {
      position: absolute;
      top: 6px; left: 8px;
      background: rgba(16, 44, 87, 0.85);
      color: #ffffff;
      font-size: 11px;
      font-weight: 700;
      padding: 2px 8px;
      border-radius: 4px;
      z-index: 2;
    }
    .no-img { font-size: 12px; color: #94a3b8; font-weight: 600; }
    .legend-box {
      background: #ffffff;
      border: 1px solid #cbd5e1;
      padding: 12px 14px;
      display: flex;
      flex-direction: column;
      gap: 6px;
      font-size: 11.5px;
      justify-content: center;
    }
    .legend-title { font-weight: 700; color: #1e293b; margin-bottom: 2px; }
    .legend-subtitle { font-weight: 700; color: #475569; margin-top: 4px; }
    .legend-row { display: flex; align-items: center; gap: 8px; }
    .color-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }

    /* Integrated Time Series Layout */
    .ts-integrated-container {
      display: flex;
      gap: 12px;
      flex: 1;
      height: calc(100% - 32px);
    }
    .ts-main-grid {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 6px;
      height: 100%;
    }
    .ts-header-row {
      display: flex;
      height: 22px;
      font-size: 13px;
      font-weight: 700;
      color: #102c57;
      text-align: center;
    }
    .ts-label-space { width: 90px; flex-shrink: 0; }
    .ts-col-header { flex: 1; }
    .ts-row {
      display: flex;
      flex: 1;
      gap: 8px;
      align-items: center;
    }
    .ts-row-label {
      width: 90px;
      flex-shrink: 0;
      font-size: 12px;
      font-weight: 700;
      color: #334155;
      text-align: right;
      padding-right: 8px;
    }
    .ts-img-box {
      flex: 1;
      height: 100%;
      background: #f8fafc;
      border: 1px solid #e2e8f0;
      border-radius: 6px;
      overflow: hidden;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .ts-img-box img { max-width: 100%; max-height: 100%; object-fit: contain; cursor: zoom-in; }
    .ts-legend-sidebar {
      width: 170px;
      background: #ffffff;
      border: 1px solid #cbd5e1;
      border-radius: 6px;
      padding: 12px;
      font-size: 11px;
      display: flex;
      flex-direction: column;
      gap: 6px;
      flex-shrink: 0;
    }

    /* Profile Layout */
    .grid-profile {
      display: grid;
      grid-template-columns: 1fr 1fr;
      grid-template-rows: 1fr 1fr;
      gap: 10px;
      flex: 1;
    }
    .grid-profile-amdar {
      display: flex;
      gap: 12px;
      flex: 1;
      height: 100%;
    }
    .grid-profile-amdar .profile-box {
      flex: 1;
      height: 100%;
    }
    .grid-profile-amdar .profile-amdar-legend {
      flex: 0 0 180px;
      height: 100%;
    }
    .profile-box {
      position: relative;
      border: 1px solid #e2e8f0;
      border-radius: 6px;
      overflow: hidden;
      display: flex;
      align-items: center;
      justify-content: center;
      background: #f8fafc;
    }
    .profile-box img { max-width: 100%; max-height: 100%; object-fit: contain; cursor: zoom-in; }
    .profile-label {
      position: absolute;
      top: 6px; left: 10px;
      background: rgba(16, 44, 87, 0.85);
      color: #ffffff;
      font-size: 11.5px;
      font-weight: 700;
      padding: 3px 8px;
      border-radius: 4px;
      z-index: 2;
    }

    /* Radar Cases Layout */
    .radar-case-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      grid-template-rows: 1fr 1fr;
      gap: 10px;
      flex: 1;
    }
    .case-box {
      position: relative;
      border: 1px solid #e2e8f0;
      border-radius: 6px;
      overflow: hidden;
      display: flex;
      align-items: center;
      justify-content: center;
      background: #f8fafc;
    }
    .case-box img { max-width: 100%; max-height: 100%; object-fit: contain; cursor: zoom-in; }
    .case-label {
      position: absolute;
      top: 4px; left: 6px;
      background: rgba(15, 23, 42, 0.85);
      color: #ffffff;
      font-size: 11px;
      font-weight: 700;
      padding: 2px 6px;
      border-radius: 4px;
      z-index: 2;
    }

    /* Lightbox Modal */
    .lightbox-modal {
      display: none;
      position: fixed;
      top: 0; left: 0; right: 0; bottom: 0;
      background: rgba(15, 23, 42, 0.95);
      z-index: 2000;
      align-items: center;
      justify-content: center;
      padding: 20px;
    }
    .lightbox-modal.active { display: flex; }
    .lightbox-modal img {
      max-width: 95vw;
      max-height: 95vh;
      object-fit: contain;
      border-radius: 6px;
      box-shadow: 0 0 30px rgba(0,0,0,0.5);
    }
    .lightbox-close {
      position: absolute;
      top: 20px; right: 24px;
      color: #ffffff;
      font-size: 32px;
      cursor: pointer;
      user-select: none;
    }

    @media print {
      .toolbar { display: none; }
      .presentation-container { margin-top: 0; padding: 0; gap: 0; }
      .slide-card { page-break-after: always; box-shadow: none; border: none; }
    }
  </style>
</head>
<body class="mode-slideshow">

  <!-- Top Navigation Bar -->
  <header class="toolbar">
    <div class="toolbar-left">
      <span class="toolbar-title">⚡ NOAA Verification Report</span>
      __REGION_TABS__
      <select id="slideSelect" class="slide-select" onchange="goToSlide(parseInt(this.value))">
        __NAV_OPTIONS__
      </select>
    </div>
    
    <div class="toolbar-center">
      <button class="tool-btn" onclick="prevSlide()">◀ Prev Slide</button>
      <span class="counter-badge" id="slideCounter">Slide 1 / __TOTAL_SLIDES__</span>
      <button class="tool-btn" onclick="nextSlide()">Next Slide ▶</button>
    </div>
    
    <div class="toolbar-right">
      <button class="tool-btn" id="btnToggleMode" onclick="toggleViewMode()">📜 Continuous Scroll</button>
      <button class="tool-btn" onclick="toggleFullscreen()">⛶ Fullscreen</button>
      <button class="tool-btn primary" onclick="window.print()">🖨️ Export PDF / Print</button>
    </div>
  </header>

  <!-- Presentation Slides -->
  <main class="presentation-container">
    __SLIDES_HTML__
  </main>

  <!-- Image Lightbox Modal -->
  <div id="lightbox" class="lightbox-modal" onclick="closeLightbox()">
    <span class="lightbox-close">&times;</span>
    <img id="lightboxImg" src="" alt="High resolution plot">
  </div>

  <script>
    let currentSlide = 1;
    const totalSlides = __TOTAL_SLIDES__;
    let isSlideshow = true;
    let isProgrammaticScroll = false;

    function updateSlideUI() {
      if (isSlideshow) {
        document.querySelectorAll('.slide-card').forEach(s => s.classList.remove('active'));
        const el = document.getElementById('slide-' + currentSlide);
        if (el) el.classList.add('active');
      } else {
        const el = document.getElementById('slide-' + currentSlide);
        if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
      document.getElementById('slideCounter').innerText = 'Slide ' + currentSlide + ' / ' + totalSlides;
      document.getElementById('slideSelect').value = currentSlide;

      // Sync active region tab with current slide
      const activeSlide = document.getElementById('slide-' + currentSlide);
      if (activeSlide) {
        const currentReg = activeSlide.getAttribute('data-region') || 'all';
        document.querySelectorAll('.region-tab').forEach(b => {
          b.classList.toggle('active', b.getAttribute('data-filter') === currentReg);
        });
      }
    }

    function goToSlide(n) {
      if (n >= 1 && n <= totalSlides) {
        currentSlide = n;
        isProgrammaticScroll = true;
        updateSlideUI();
        setTimeout(() => { isProgrammaticScroll = false; }, 700);
      }
    }

    function filterRegion(reg, btn) {
      if (btn) {
        document.querySelectorAll('.region-tab').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
      }
      const target = document.querySelector('.slide-card[data-region="' + reg + '"]');
      if (target) {
        const slideNum = parseInt(target.getAttribute('data-slide-num'));
        goToSlide(slideNum);
      }
    }

    function nextSlide() {
      if (currentSlide < totalSlides) {
        currentSlide++;
        updateSlideUI();
      }
    }

    function prevSlide() {
      if (currentSlide > 1) {
        currentSlide--;
        updateSlideUI();
      }
    }

    function toggleViewMode() {
      isSlideshow = !isSlideshow;
      const btn = document.getElementById('btnToggleMode');
      if (isSlideshow) {
        document.body.className = 'mode-slideshow';
        btn.innerText = '📜 Continuous Scroll';
        btn.classList.remove('active');
      } else {
        document.body.className = 'mode-scroll';
        btn.innerText = '🖥️ Slideshow Mode';
        btn.classList.add('active');
      }
      updateSlideUI();
    }

    function toggleFullscreen() {
      if (!document.fullscreenElement) {
        document.documentElement.requestFullscreen().catch(() => {});
      } else {
        document.exitFullscreen().catch(() => {});
      }
    }

    const scrollObserver = new IntersectionObserver((entries) => {
      if (isSlideshow || isProgrammaticScroll) return;
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          const sNum = parseInt(entry.target.getAttribute('data-slide-num'));
          if (sNum && sNum !== currentSlide) {
            currentSlide = sNum;
            document.getElementById('slideCounter').innerText = 'Slide ' + currentSlide + ' / ' + totalSlides;
            document.getElementById('slideSelect').value = currentSlide;

            const reg = entry.target.getAttribute('data-region') || 'all';
            document.querySelectorAll('.region-tab').forEach(b => {
              b.classList.toggle('active', b.getAttribute('data-filter') === reg);
            });
          }
        }
      });
    }, { threshold: 0.5 });

    document.querySelectorAll('.slide-card').forEach(card => scrollObserver.observe(card));

    document.addEventListener('keydown', (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT') return;
      if (e.key === 'ArrowRight' || e.key === 'Space' || e.key === 'PageDown') {
        nextSlide();
      } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
        prevSlide();
      } else if (e.key === 'f' || e.key === 'F') {
        toggleFullscreen();
      } else if (e.key === 'Escape') {
        closeLightbox();
      }
    });

    function openLightbox(img) {
      if (!img || !img.src) return;
      const lb = document.getElementById('lightbox');
      const lbImg = document.getElementById('lightboxImg');
      lbImg.src = img.src;
      lb.classList.add('active');
    }

    function closeLightbox() {
      document.getElementById('lightbox').classList.remove('active');
    }
  </script>
</body>
</html>
`
