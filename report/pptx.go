package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goppt "github.com/casibase/goppt"
	"verification_studio/mats"
)

// BuildPresentation compiles the PowerPoint presentation (.pptx) deck in pure Go.
// Returns the collected SlideData for exact 1:1 synchronization with the interactive HTML report.
func BuildPresentation(cfg Config, matsDir, outputPptx string) ([]SlideData, error) {
	fmt.Printf("\n>>> Compiling PowerPoint presentation: %s...\n", outputPptx)

	prs := goppt.New()
	layout := prs.GetLayout()
	if layout == nil {
		layout = goppt.NewDocumentLayout()
	}
	layout.SetCustomLayout(9144000, 5143500) // 16:9 widescreen: 10in x 5.625in
	prs.SetLayout(layout)

	models := cfg.Models
	titleText := cfg.Title
	if titleText == "" {
		titleText = "Numerical Weather Prediction Verification Report"
	}
	crefDir := cfg.CrefDir
	if crefDir == "" {
		crefDir = filepath.Join(filepath.Dir(outputPptx), "cref")
	}

	var slidesData []SlideData

	findImg := func(fn string) string {
		if fn == "" {
			return ""
		}
		p := filepath.Join(matsDir, fn)
		if isFileValid(p) {
			return p
		}
		if strings.Contains(fn, "ft.") {
			pAlt := filepath.Join(matsDir, strings.ReplaceAll(fn, "ft.", "."))
			if isFileValid(pAlt) {
				return pAlt
			}
		}
		if !strings.Contains(fn, "ft.") && (strings.Contains(fn, "500.") || strings.Contains(fn, "1000.") || strings.Contains(fn, "3000.")) {
			pAlt := filepath.Join(matsDir, strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(fn, "500.", "500ft."), "1000.", "1000ft."), "3000.", "3000ft."))
			if isFileValid(pAlt) {
				return pAlt
			}
		}
		return ""
	}

	addHeader := func(slide *goppt.Slide, title string) {
		tx := slide.AddTextBox()
		tx.SetOffsetX(goppt.Inch(0.5)).SetOffsetY(goppt.Inch(0.12)).SetWidth(goppt.Inch(9.0)).SetHeight(goppt.Inch(0.45))
		tx.SetWordWrap(true)
		p := tx.GetActiveParagraph()
		if p == nil {
			p = tx.CreateParagraph()
		}
		p.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
		tr := p.CreateTextRun(title)
		tr.GetFont().SetSize(21).SetBold(true).SetColor(goppt.NewColor("FF102C57"))
	}

	addLegendBox := func(slide *goppt.Slide, isBias bool, metric string) {
		tx := slide.AddTextBox()
		tx.SetOffsetX(7313400).SetOffsetY(2958262).SetWidth(1650000).SetHeight(1600000)
		tx.SetWordWrap(true)

		type textSpec struct {
			Text string
			Bold bool
			Size int
		}
		var lines []textSpec
		lines = append(lines, textSpec{Text: "Experiments:", Bold: true, Size: 11})
		for _, m := range models {
			lines = append(lines, textSpec{Text: "• " + m.Name, Bold: false, Size: 10})
		}
		if isBias {
			lines = append(lines, textSpec{Text: "Metric:", Bold: true, Size: 11})
			lines = append(lines, textSpec{Text: "• Dash: BIAS", Bold: false, Size: 10})
		} else if metric == "COMBINED" {
			lines = append(lines, textSpec{Text: "Styles:", Bold: true, Size: 11})
			lines = append(lines, textSpec{Text: "• Solid: RMSE", Bold: false, Size: 10})
			lines = append(lines, textSpec{Text: "• Dash: BIAS", Bold: false, Size: 10})
		} else {
			lines = append(lines, textSpec{Text: "Metric:", Bold: true, Size: 11})
			lines = append(lines, textSpec{Text: "• Solid: " + metric, Bold: false, Size: 10})
		}

		for idx, spec := range lines {
			var p *goppt.Paragraph
			if idx == 0 {
				p = tx.GetActiveParagraph()
				if p == nil {
					p = tx.CreateParagraph()
				}
			} else {
				p = tx.CreateParagraph()
			}
			tr := p.CreateTextRun(spec.Text)
			tr.GetFont().SetSize(spec.Size).SetBold(spec.Bold)
		}
	}

	add3PanelSlide := func(title, f1, f2, f3, l1, l2, l3 string, isBias bool, metric, region string) bool {
		p1 := findImg(f1)
		p2 := findImg(f2)
		p3 := findImg(f3)
		if p1 == "" && p2 == "" && p3 == "" {
			return false
		}

		slide := prs.CreateSlide()
		addHeader(slide, title)

		if p1 != "" {
			img := slide.CreateDrawingShape()
			img.SetPath(p1).SetOffsetX(216610).SetOffsetY(705325).SetWidth(4244023).SetHeight(2127095)
			lbl := slide.AddTextBox()
			lbl.SetOffsetX(1886891).SetOffsetY(355817).SetWidth(1200000).SetHeight(350000)
			p := lbl.GetActiveParagraph()
			if p == nil {
				p = lbl.CreateParagraph()
			}
			tr := p.CreateTextRun(l1)
			tr.GetFont().SetSize(14).SetBold(true)
		}

		if p2 != "" {
			img := slide.CreateDrawingShape()
			img.SetPath(p2).SetOffsetX(4506775).SetOffsetY(714750).SetWidth(4529823).SetHeight(2127100)
			lbl := slide.AddTextBox()
			lbl.SetOffsetX(5388838).SetOffsetY(378849).SetWidth(1200000).SetHeight(350000)
			p := lbl.GetActiveParagraph()
			if p == nil {
				p = lbl.CreateParagraph()
			}
			tr := p.CreateTextRun(l2)
			tr.GetFont().SetSize(14).SetBold(true)
		}

		if p3 != "" {
			img := slide.CreateDrawingShape()
			img.SetPath(p3).SetOffsetX(2365625).SetOffsetY(2894825).SetWidth(4529823).SetHeight(2243827)
			lbl := slide.AddTextBox()
			lbl.SetOffsetX(1468986).SetOffsetY(3620929).SetWidth(1200000).SetHeight(350000)
			p := lbl.GetActiveParagraph()
			if p == nil {
				p = lbl.CreateParagraph()
			}
			tr := p.CreateTextRun(l3)
			tr.GetFont().SetSize(14).SetBold(true)
		}

		addLegendBox(slide, isBias, metric)

		slidesData = append(slidesData, SlideData{
			Type:   "3panel",
			Title:  title,
			Img1:   p1,
			Lbl1:   l1,
			Img2:   p2,
			Lbl2:   l2,
			Img3:   p3,
			Lbl3:   l3,
			IsBias: isBias,
			Metric: metric,
			Models: models,
			Region: region,
		})
		return true
	}

	addIntegratedTSSlide := func(title, col1Name, col2Name string, rowSpecs [][3]string, region string) bool {
		var activeRows []RowItem
		for _, spec := range rowSpecs {
			lbl := spec[0]
			p1 := findImg(spec[1])
			p2 := findImg(spec[2])
			if p1 != "" || p2 != "" {
				activeRows = append(activeRows, RowItem{Label: lbl, Img1: p1, Img2: p2})
			}
		}

		if len(activeRows) == 0 {
			return false
		}

		slide := prs.CreateSlide()
		addHeader(slide, title)

		xLbl := int64(0.20 * 914400)
		wLbl := int64(1.15 * 914400)
		xCol1 := int64(1.40 * 914400)
		wCol := int64(3.35 * 914400)
		xCol2 := int64(4.85 * 914400)
		xLeg := int64(8.30 * 914400)
		wLeg := int64(1.65 * 914400)
		yHdr := int64(0.55 * 914400)
		hHdr := int64(0.28 * 914400)

		// Column header 1
		t1 := slide.AddTextBox()
		t1.SetOffsetX(xCol1).SetOffsetY(yHdr).SetWidth(wCol).SetHeight(hHdr)
		p1 := t1.GetActiveParagraph()
		if p1 == nil {
			p1 = t1.CreateParagraph()
		}
		p1.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
		tr1 := p1.CreateTextRun(col1Name)
		tr1.GetFont().SetSize(15).SetBold(true)

		// Column header 2
		t2 := slide.AddTextBox()
		t2.SetOffsetX(xCol2).SetOffsetY(yHdr).SetWidth(wCol).SetHeight(hHdr)
		p2 := t2.GetActiveParagraph()
		if p2 == nil {
			p2 = t2.CreateParagraph()
		}
		p2.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
		tr2 := p2.CreateTextRun(col2Name)
		tr2.GetFont().SetSize(15).SetBold(true)

		hPic := int64(1.44 * 914400)
		gapY := int64(0.06 * 914400)
		yStart := int64(0.88 * 914400)

		for idx, r := range activeRows {
			yPos := yStart + int64(idx)*(hPic+gapY)

			lblBox := slide.AddTextBox()
			lblBox.SetOffsetX(xLbl).SetOffsetY(yPos + hPic/2 - int64(0.18*914400)).SetWidth(wLbl).SetHeight(int64(0.36 * 914400))
			lp := lblBox.GetActiveParagraph()
			if lp == nil {
				lp = lblBox.CreateParagraph()
			}
			lp.GetAlignment().SetHorizontal(goppt.HorizontalRight)
			ltr := lp.CreateTextRun(r.Label)
			ltr.GetFont().SetSize(13).SetBold(true)

			if r.Img1 != "" {
				img := slide.CreateDrawingShape()
				img.SetPath(r.Img1).SetOffsetX(xCol1).SetOffsetY(yPos).SetWidth(wCol).SetHeight(hPic)
			}
			if r.Img2 != "" {
				img := slide.CreateDrawingShape()
				img.SetPath(r.Img2).SetOffsetX(xCol2).SetOffsetY(yPos).SetWidth(wCol).SetHeight(hPic)
			}
		}

		// Legend sidebar
		leg := slide.AddTextBox()
		leg.SetOffsetX(xLeg).SetOffsetY(int64(1.10 * 914400)).SetWidth(wLeg).SetHeight(int64(3.80 * 914400))
		leg.SetWordWrap(true)

		type textSpec struct {
			Text string
			Bold bool
			Size int
		}
		var lines []textSpec
		lines = append(lines, textSpec{Text: "Experiments:", Bold: true, Size: 11})
		for _, m := range models {
			lines = append(lines, textSpec{Text: "• " + m.Name, Bold: false, Size: 10})
		}
		lines = append(lines,
			textSpec{Text: "Columns:", Bold: true, Size: 11},
			textSpec{Text: "• Left: " + col1Name, Bold: false, Size: 10},
			textSpec{Text: "• Right: " + col2Name, Bold: false, Size: 10},
			textSpec{Text: "Line Styles:", Bold: true, Size: 11},
			textSpec{Text: "• Solid: " + col1Name, Bold: false, Size: 10},
			textSpec{Text: "• Dash: " + col2Name, Bold: false, Size: 10},
		)

		for idx, spec := range lines {
			var p *goppt.Paragraph
			if idx == 0 {
				p = leg.GetActiveParagraph()
				if p == nil {
					p = leg.CreateParagraph()
				}
			} else {
				p = leg.CreateParagraph()
			}
			tr := p.CreateTextRun(spec.Text)
			tr.GetFont().SetSize(spec.Size).SetBold(spec.Bold)
		}

		slidesData = append(slidesData, SlideData{
			Type:     "integrated_ts",
			Title:    title,
			Col1Name: col1Name,
			Col2Name: col2Name,
			Rows:     activeRows,
			Models:   models,
			Region:   region,
		})
		return true
	}

	addProfileSlide := func(title, fT, fW, fRH, region string) bool {
		pT := findImg(fT)
		pW := findImg(fW)
		pRH := findImg(fRH)
		if pT == "" && pW == "" && pRH == "" {
			return false
		}

		slide := prs.CreateSlide()
		addHeader(slide, title)

		if pRH != "" {
			// 3-plot RAOBs: 2x2 layout
			c1X := goppt.Inch(2.35)
			c2X := goppt.Inch(5.15)
			pSize := goppt.Inch(2.00)

			// Row 1: Temp & Wind
			if pT != "" {
				lbl1 := slide.AddTextBox()
				lbl1.SetOffsetX(c1X).SetOffsetY(goppt.Inch(0.56)).SetWidth(pSize).SetHeight(goppt.Inch(0.24))
				p := lbl1.GetActiveParagraph()
				if p == nil {
					p = lbl1.CreateParagraph()
				}
				p.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
				tr := p.CreateTextRun("Temperature (t)")
				tr.GetFont().SetSize(14).SetBold(true)

				img := slide.CreateDrawingShape()
				img.SetPath(pT).SetOffsetX(c1X).SetOffsetY(goppt.Inch(0.80)).SetWidth(pSize).SetHeight(pSize)
			}
			if pW != "" {
				lbl2 := slide.AddTextBox()
				lbl2.SetOffsetX(c2X).SetOffsetY(goppt.Inch(0.56)).SetWidth(pSize).SetHeight(goppt.Inch(0.24))
				p := lbl2.GetActiveParagraph()
				if p == nil {
					p = lbl2.CreateParagraph()
				}
				p.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
				tr := p.CreateTextRun("Wind")
				tr.GetFont().SetSize(14).SetBold(true)

				img := slide.CreateDrawingShape()
				img.SetPath(pW).SetOffsetX(c2X).SetOffsetY(goppt.Inch(0.80)).SetWidth(pSize).SetHeight(pSize)
			}

			// Row 2: RH & Legend
			lbl3 := slide.AddTextBox()
			lbl3.SetOffsetX(c1X).SetOffsetY(goppt.Inch(2.95)).SetWidth(pSize).SetHeight(goppt.Inch(0.24))
			p := lbl3.GetActiveParagraph()
			if p == nil {
				p = lbl3.CreateParagraph()
			}
			p.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
			rhLabel := "Relative Humidity (RHobT)"
			if strings.Contains(title, "AMDAR") {
				rhLabel = "Relative Humidity (RH)"
			}
			tr := p.CreateTextRun(rhLabel)
			tr.GetFont().SetSize(14).SetBold(true)

			img := slide.CreateDrawingShape()
			img.SetPath(pRH).SetOffsetX(c1X).SetOffsetY(goppt.Inch(3.20)).SetWidth(pSize).SetHeight(pSize)

			leg := slide.AddTextBox()
			leg.SetOffsetX(c2X).SetOffsetY(goppt.Inch(3.20)).SetWidth(goppt.Inch(2.45)).SetHeight(goppt.Inch(1.95))
			leg.SetWordWrap(true)
			renderProfileLegend(leg, models)
		} else {
			// 2-plot AMDAR: 3 columns side-by-side
			c1X := goppt.Inch(0.50)
			c2X := goppt.Inch(4.20)
			c3X := goppt.Inch(7.90)
			pSize := goppt.Inch(3.45)

			if pT != "" {
				lbl1 := slide.AddTextBox()
				lbl1.SetOffsetX(c1X).SetOffsetY(goppt.Inch(0.58)).SetWidth(pSize).SetHeight(goppt.Inch(0.28))
				p := lbl1.GetActiveParagraph()
				if p == nil {
					p = lbl1.CreateParagraph()
				}
				p.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
				tr := p.CreateTextRun("Temperature (t)")
				tr.GetFont().SetSize(14).SetBold(true)

				img := slide.CreateDrawingShape()
				img.SetPath(pT).SetOffsetX(c1X).SetOffsetY(goppt.Inch(0.88)).SetWidth(pSize).SetHeight(pSize)
			}
			if pW != "" {
				lbl2 := slide.AddTextBox()
				lbl2.SetOffsetX(c2X).SetOffsetY(goppt.Inch(0.58)).SetWidth(pSize).SetHeight(goppt.Inch(0.28))
				p := lbl2.GetActiveParagraph()
				if p == nil {
					p = lbl2.CreateParagraph()
				}
				p.GetAlignment().SetHorizontal(goppt.HorizontalCenter)
				tr := p.CreateTextRun("Wind")
				tr.GetFont().SetSize(14).SetBold(true)

				img := slide.CreateDrawingShape()
				img.SetPath(pW).SetOffsetX(c2X).SetOffsetY(goppt.Inch(0.88)).SetWidth(pSize).SetHeight(pSize)
			}

			leg := slide.AddTextBox()
			leg.SetOffsetX(c3X).SetOffsetY(goppt.Inch(0.88)).SetWidth(goppt.Inch(1.65)).SetHeight(pSize)
			leg.SetWordWrap(true)
			renderProfileLegend(leg, models)
		}

		slidesData = append(slidesData, SlideData{
			Type:   "profile",
			Title:  title,
			ImgT:   pT,
			ImgW:   pW,
			ImgRH:  pRH,
			Models: models,
			Region: region,
		})
		return true
	}

	addSectionDividerSlide := func(regID string) {
		slide := prs.CreateSlide()
		bg := slide.AddAutoShape()
		bg.SetAutoShapeType(goppt.AutoShapeRectangle)
		bg.SetSolidFill(goppt.NewColor("FF102C57"))
		bg.SetOffsetX(0).SetOffsetY(0).SetWidth(9144000).SetHeight(5143500)

		regFullName := mats.RegionNameMap[regID]
		if regFullName == "" {
			regFullName = strings.Title(regID)
		}

		// Pill Header
		tbPill := slide.AddTextBox()
		tbPill.SetOffsetX(goppt.Inch(1.0)).SetOffsetY(goppt.Inch(1.2)).SetWidth(goppt.Inch(8.0)).SetHeight(goppt.Inch(0.45))
		pPill := tbPill.GetActiveParagraph()
		if pPill == nil {
			pPill = tbPill.CreateParagraph()
		}
		trPill := pPill.CreateTextRun("REGIONAL VERIFICATION CHAPTER")
		trPill.GetFont().SetSize(13).SetBold(true).SetColor(goppt.NewColor("FF38BDF8"))

		// Main Title
		tbTitle := slide.AddTextBox()
		tbTitle.SetOffsetX(goppt.Inch(1.0)).SetOffsetY(goppt.Inch(1.75)).SetWidth(goppt.Inch(8.0)).SetHeight(goppt.Inch(1.3))
		pTitle := tbTitle.GetActiveParagraph()
		if pTitle == nil {
			pTitle = tbTitle.CreateParagraph()
		}
		trTitle := pTitle.CreateTextRun(fmt.Sprintf("Sub-Domain Evaluation:\n%s", regFullName))
		trTitle.GetFont().SetSize(26).SetBold(true).SetColor(goppt.NewColor("FFFFFFFF"))

		// Subtitle
		tbSub := slide.AddTextBox()
		tbSub.SetOffsetX(goppt.Inch(1.0)).SetOffsetY(goppt.Inch(3.25)).SetWidth(goppt.Inch(8.0)).SetHeight(goppt.Inch(0.8))
		pSub := tbSub.GetActiveParagraph()
		if pSub == nil {
			pSub = tbSub.CreateParagraph()
		}
		trSub := pSub.CreateTextRun(fmt.Sprintf("Comparative performance analysis across %s for Surface, Upper Air, Radar, and Aviation METARs.", regFullName))
		trSub.GetFont().SetSize(13).SetColor(goppt.NewColor("FFCBD5E1"))

		slidesData = append(slidesData, SlideData{
			Type:     "section_header",
			Region:   regID,
			Title:    fmt.Sprintf("Sub-Domain Evaluation: %s", regFullName),
			Subtitle: fmt.Sprintf("Comparative performance analysis across %s.", regFullName),
		})
	}

	addRegionalVerificationDeck := func(reg string) {
		regTag := ""
		if reg != "all" {
			regTag = "_" + reg
		}
		regSuffix := ""
		if reg != "all" {
			name := mats.RegionNameMap[reg]
			if name == "" {
				name = reg
			}
			regSuffix = " (" + strings.Split(name, " (")[0] + ")"
		}

		// 1. Surface Dieoff
		add3PanelSlide(
			fmt.Sprintf("Surface Dieoff%s", regSuffix),
			fmt.Sprintf("surface_dieoff_t2m%s.png", regTag),
			fmt.Sprintf("surface_dieoff_td2m%s.png", regTag),
			fmt.Sprintf("surface_dieoff_wind10m%s.png", regTag),
			"t2m", "td2m", "wind10m",
			false, "COMBINED", reg,
		)

		// 2. Surface Time Series
		for _, item := range []struct {
			Title string
			Flead string
		}{
			{Title: fmt.Sprintf("Surface 1-h fcst time series%s", regSuffix), Flead: "1h"},
			{Title: fmt.Sprintf("Surface 3-h fcst time series%s", regSuffix), Flead: "3h"},
			{Title: fmt.Sprintf("Surface 12-h fcst time series%s", regSuffix), Flead: "12h"},
			{Title: fmt.Sprintf("Surface 24-h fcst time series%s", regSuffix), Flead: "24h"},
		} {
			addIntegratedTSSlide(
				item.Title, "RMSE", "BIAS",
				[][3]string{
					{"t2m", fmt.Sprintf("sfc_ts_%s_rmse_t2m%s.png", item.Flead, regTag), fmt.Sprintf("sfc_ts_%s_bias_t2m%s.png", item.Flead, regTag)},
					{"td2m", fmt.Sprintf("sfc_ts_%s_rmse_td2m%s.png", item.Flead, regTag), fmt.Sprintf("sfc_ts_%s_bias_td2m%s.png", item.Flead, regTag)},
					{"Wind 10m", fmt.Sprintf("sfc_ts_%s_rmse_wind10m%s.png", item.Flead, regTag), fmt.Sprintf("sfc_ts_%s_bias_wind10m%s.png", item.Flead, regTag)},
				},
				reg,
			)
		}

		// 3. Upper Air RAOBs Dieoff
		add3PanelSlide(
			fmt.Sprintf("Upper air against RAOBs dieoff%s", regSuffix),
			fmt.Sprintf("ua_dieoff_temp%s.png", regTag),
			fmt.Sprintf("ua_dieoff_rh%s.png", regTag),
			fmt.Sprintf("ua_dieoff_wind%s.png", regTag),
			"Temperature", "RHobT", "Wind",
			false, "COMBINED", reg,
		)

		// AMDAR Dieoff (full domain only)
		if reg == "all" {
			add3PanelSlide(
				"Upper air against AMDAR dieoff",
				"amdar_dieoff_temp.png", "amdar_dieoff_rh.png", "amdar_dieoff_wind.png",
				"Temperature", "RH", "Wind",
				false, "COMBINED", reg,
			)
		}

		// 4. Upper Air RAOBs Profiles (1h, 3h, 6h, 12h)
		addProfileSlide(fmt.Sprintf("Upper air 1-h fcst (RAOBs Profile)%s", regSuffix), fmt.Sprintf("ua_prof_1h_temp%s.png", regTag), fmt.Sprintf("ua_prof_1h_wind%s.png", regTag), fmt.Sprintf("ua_prof_1h_rh%s.png", regTag), reg)
		addProfileSlide(fmt.Sprintf("Upper air 3-h fcst (RAOBs Profile)%s", regSuffix), fmt.Sprintf("ua_prof_3h_temp%s.png", regTag), fmt.Sprintf("ua_prof_3h_wind%s.png", regTag), fmt.Sprintf("ua_prof_3h_rh%s.png", regTag), reg)
		addProfileSlide(fmt.Sprintf("Upper air 6-h fcst (RAOBs Profile)%s", regSuffix), fmt.Sprintf("ua_prof_6h_temp%s.png", regTag), fmt.Sprintf("ua_prof_6h_wind%s.png", regTag), fmt.Sprintf("ua_prof_6h_rh%s.png", regTag), reg)
		addProfileSlide(fmt.Sprintf("Upper air 12-h fcst (RAOBs Profile)%s", regSuffix), fmt.Sprintf("ua_prof_12h_temp%s.png", regTag), fmt.Sprintf("ua_prof_12h_wind%s.png", regTag), fmt.Sprintf("ua_prof_12h_rh%s.png", regTag), reg)

		// AMDAR Profiles (full domain only)
		if reg == "all" {
			addProfileSlide("Upper air 1-h fcst (AMDAR Profile)", "amdar_prof_1h_temp.png", "amdar_prof_1h_wind.png", "amdar_prof_1h_rh.png", reg)
			addProfileSlide("Upper air 3-h fcst (AMDAR Profile)", "amdar_prof_3h_temp.png", "amdar_prof_3h_wind.png", "amdar_prof_3h_rh.png", reg)
			addProfileSlide("Upper air 6-h fcst (AMDAR Profile)", "amdar_prof_6h_temp.png", "amdar_prof_6h_wind.png", "amdar_prof_6h_rh.png", reg)
			addProfileSlide("Upper air 12-h fcst (AMDAR Profile)", "amdar_prof_12h_temp.png", "amdar_prof_12h_wind.png", "amdar_prof_12h_rh.png", reg)
		}

		// 5. Radar Reflectivity
		add3PanelSlide(fmt.Sprintf("CREF Dieoff (CSI )%s", regSuffix), fmt.Sprintf("ref_dieoff_csi_15dbz%s.png", regTag), fmt.Sprintf("ref_dieoff_csi_25dbz%s.png", regTag), fmt.Sprintf("ref_dieoff_csi_35dbz%s.png", regTag), "15 dBZ", "25 dBZ", "35 dBZ", false, "CSI", reg)
		add3PanelSlide(fmt.Sprintf("CREF Dieoff (BIAS )%s", regSuffix), fmt.Sprintf("ref_dieoff_bias_15dbz%s.png", regTag), fmt.Sprintf("ref_dieoff_bias_25dbz%s.png", regTag), fmt.Sprintf("ref_dieoff_bias_35dbz%s.png", regTag), "15 dBZ", "25 dBZ", "35 dBZ", true, "BIAS", reg)
		for _, item := range []struct {
			Title string
			Flead string
		}{
			{Title: fmt.Sprintf("CREF 1-h fcst time series%s", regSuffix), Flead: "1h"},
			{Title: fmt.Sprintf("CREF 3-h fcst time series%s", regSuffix), Flead: "3h"},
		} {
			addIntegratedTSSlide(
				item.Title, "CSI", "BIAS",
				[][3]string{
					{"15 dBZ", fmt.Sprintf("ref_ts_%s_csi_15dbz%s.png", item.Flead, regTag), fmt.Sprintf("ref_ts_%s_bias_15dbz%s.png", item.Flead, regTag)},
					{"25 dBZ", fmt.Sprintf("ref_ts_%s_csi_25dbz%s.png", item.Flead, regTag), fmt.Sprintf("ref_ts_%s_bias_25dbz%s.png", item.Flead, regTag)},
					{"35 dBZ", fmt.Sprintf("ref_ts_%s_csi_35dbz%s.png", item.Flead, regTag), fmt.Sprintf("ref_ts_%s_bias_35dbz%s.png", item.Flead, regTag)},
				},
				reg,
			)
		}

		// 6. Cloud Ceiling
		add3PanelSlide(fmt.Sprintf("Ceiling Dieoff (CSI)%s", regSuffix), fmt.Sprintf("ceil_dieoff_csi_500ft%s.png", regTag), fmt.Sprintf("ceil_dieoff_csi_1000ft%s.png", regTag), fmt.Sprintf("ceil_dieoff_csi_3000ft%s.png", regTag), "< 500 ft", "< 1000 ft", "< 3000 ft", false, "CSI", reg)
		add3PanelSlide(fmt.Sprintf("Ceiling Dieoff (BIAS)%s", regSuffix), fmt.Sprintf("ceil_dieoff_bias_500ft%s.png", regTag), fmt.Sprintf("ceil_dieoff_bias_1000ft%s.png", regTag), fmt.Sprintf("ceil_dieoff_bias_3000ft%s.png", regTag), "< 500 ft", "< 1000 ft", "< 3000 ft", true, "BIAS", reg)
		for _, item := range []struct {
			Title string
			Flead string
		}{
			{Title: fmt.Sprintf("Ceiling 1-h fcst time series%s", regSuffix), Flead: "1h"},
			{Title: fmt.Sprintf("Ceiling 3-h fcst time series%s", regSuffix), Flead: "3h"},
		} {
			addIntegratedTSSlide(
				item.Title, "CSI", "BIAS",
				[][3]string{
					{"< 500 ft", fmt.Sprintf("ceil_ts_%s_csi_500ft%s.png", item.Flead, regTag), fmt.Sprintf("ceil_ts_%s_bias_500ft%s.png", item.Flead, regTag)},
					{"< 1000 ft", fmt.Sprintf("ceil_ts_%s_csi_1000ft%s.png", item.Flead, regTag), fmt.Sprintf("ceil_ts_%s_bias_1000ft%s.png", item.Flead, regTag)},
					{"< 3000 ft", fmt.Sprintf("ceil_ts_%s_csi_3000ft%s.png", item.Flead, regTag), fmt.Sprintf("ceil_ts_%s_bias_3000ft%s.png", item.Flead, regTag)},
				},
				reg,
			)
		}

		// 7. Visibility
		add3PanelSlide(fmt.Sprintf("Visibility Dieoff (CSI)%s", regSuffix), fmt.Sprintf("vis_dieoff_csi_1mi%s.png", regTag), fmt.Sprintf("vis_dieoff_csi_3mi%s.png", regTag), fmt.Sprintf("vis_dieoff_csi_5mi%s.png", regTag), "< 1 mi", "< 3 mi", "< 5 mi", false, "CSI", reg)
		add3PanelSlide(fmt.Sprintf("Visibility Dieoff (BIAS)%s", regSuffix), fmt.Sprintf("vis_dieoff_bias_1mi%s.png", regTag), fmt.Sprintf("vis_dieoff_bias_3mi%s.png", regTag), fmt.Sprintf("vis_dieoff_bias_5mi%s.png", regTag), "< 1 mi", "< 3 mi", "< 5 mi", true, "BIAS", reg)
		for _, item := range []struct {
			Title string
			Flead string
		}{
			{Title: fmt.Sprintf("Visibility 1-h fcst time series%s", regSuffix), Flead: "1h"},
			{Title: fmt.Sprintf("Visibility 3-h fcst time series%s", regSuffix), Flead: "3h"},
		} {
			addIntegratedTSSlide(
				item.Title, "CSI", "BIAS",
				[][3]string{
					{"< 1 mi", fmt.Sprintf("vis_ts_%s_csi_1mi%s.png", item.Flead, regTag), fmt.Sprintf("vis_ts_%s_bias_1mi%s.png", item.Flead, regTag)},
					{"< 3 mi", fmt.Sprintf("vis_ts_%s_csi_3mi%s.png", item.Flead, regTag), fmt.Sprintf("vis_ts_%s_bias_3mi%s.png", item.Flead, regTag)},
					{"< 5 mi", fmt.Sprintf("vis_ts_%s_csi_5mi%s.png", item.Flead, regTag), fmt.Sprintf("vis_ts_%s_bias_5mi%s.png", item.Flead, regTag)},
				},
				reg,
			)
		}
	}

	regions := cfg.Regions
	if len(regions) == 0 {
		regions = []string{"all"}
	}

	// -------------------------------------------------------------
	// Slide 1: Title & Overview
	// Reuse slide 0 created by goppt.New() to prevent empty slide 1
	// -------------------------------------------------------------
	s1, err := prs.GetSlide(0)
	if err != nil || s1 == nil {
		s1 = prs.CreateSlide()
	}
	tb := s1.AddTextBox()
	tb.SetOffsetX(goppt.Inch(0.6)).SetOffsetY(goppt.Inch(0.4)).SetWidth(goppt.Inch(8.8)).SetHeight(goppt.Inch(0.8))
	p := tb.GetActiveParagraph()
	if p == nil {
		p = tb.CreateParagraph()
	}
	tr := p.CreateTextRun(titleText)
	tr.GetFont().SetSize(21).SetBold(true).SetColor(goppt.NewColor("FF102C57"))

	bBox := s1.AddTextBox()
	bBox.SetOffsetX(goppt.Inch(0.6)).SetOffsetY(goppt.Inch(1.3)).SetWidth(goppt.Inch(8.8)).SetHeight(goppt.Inch(3.5))
	bBox.SetWordWrap(true)

	var bullets []BulletItem
	bullets = append(bullets, BulletItem{Text: "Experiments Evaluated:", IsBold: true})
	for _, m := range models {
		bullets = append(bullets, BulletItem{Text: fmt.Sprintf("%s (%s)", m.Name, m.ID), IsBold: false})
	}
	var regNames []string
	for _, r := range regions {
		n := mats.RegionNameMap[r]
		if n == "" {
			n = r
		}
		regNames = append(regNames, strings.Split(n, " (")[0])
	}
	bullets = append(bullets, BulletItem{Text: fmt.Sprintf("Evaluated Domains: %s", strings.Join(regNames, ", ")), IsBold: true})
	bullets = append(bullets,
		BulletItem{Text: "Verification Sources:", IsBold: true},
		BulletItem{Text: "METAR surface station observations (Plot Matched)", IsBold: false},
		BulletItem{Text: "Traditional Rawinsonde (RAOBs) soundings from 1000 hPa to 100 hPa", IsBold: false},
		BulletItem{Text: "Cloud Ceiling & Surface Visibility (METAR thresholds)", IsBold: false},
	)
	if findImg("ref_dieoff_csi_15dbz.png") != "" || len(cfg.RadarCases) > 0 {
		bullets = append(bullets, BulletItem{Text: "Composite Reflectivity (MRMS Radar at 15, 25, 35 dBZ)", IsBold: false})
	}
	if findImg("amdar_dieoff_temp.png") != "" {
		bullets = append(bullets, BulletItem{Text: "Aircraft observations (AMDAR traditional)", IsBold: false})
	}

	for idx, b := range bullets {
		var bp *goppt.Paragraph
		if idx == 0 {
			bp = bBox.GetActiveParagraph()
			if bp == nil {
				bp = bBox.CreateParagraph()
			}
		} else {
			bp = bBox.CreateParagraph()
		}
		prefix := ""
		if !b.IsBold {
			prefix = "  • "
		}
		btr := bp.CreateTextRun(prefix + b.Text)
		btr.GetFont().SetSize(13).SetBold(b.IsBold)
	}

	slidesData = append(slidesData, SlideData{
		Type:    "title",
		Title:   titleText,
		Models:  models,
		Bullets: bullets,
		Dates:   cfg.Dates,
		Regions: regions,
		Region:  "all",
	})

	// -------------------------------------------------------------
	// Full Domain Verification Deck
	// -------------------------------------------------------------
	addRegionalVerificationDeck("all")

	// -------------------------------------------------------------
	// Additional Regional Sub-Domains
	// -------------------------------------------------------------
	for _, reg := range regions {
		if reg != "all" {
			addSectionDividerSlide(reg)
			addRegionalVerificationDeck(reg)
		}
	}

	// -------------------------------------------------------------
	// Synoptic Radar Reflectivity Comparison Cases
	// -------------------------------------------------------------
	if !cfg.SkipCref && len(cfg.RadarCases) > 0 {
		w := int64(2.85 * 914400)
		h := int64(2.15 * 914400)
		yTop := int64(0.55 * 914400)
		yBot := int64(2.85 * 914400)
		x1, x2, x3 := int64(0.4*914400), int64(3.4*914400), int64(6.4*914400)
		gridCoords := [][2]int64{
			{x1, yTop}, {x2, yTop}, {x3, yTop},
			{x1, yBot}, {x2, yBot}, {x3, yBot},
		}

		for _, cs := range cfg.RadarCases {
			mrmsFn := fmt.Sprintf("mrms_%s_%sz_f%02d.png", cs.Date, cs.Cycle, cs.Fhr)
			pMrms := filepath.Join(crefDir, mrmsFn)
			if !isFileValid(pMrms) {
				pMrms = ""
			}

			cards := []CardItem{
				{Name: "MRMS (Observation)", ImgPath: pMrms},
			}

			for _, m := range models {
				mFn := fmt.Sprintf("%s_%s_%sz_f%02d.png", m.Name, cs.Date, cs.Cycle, cs.Fhr)
				mPath := filepath.Join(crefDir, mFn)
				if !isFileValid(mPath) {
					mPath = ""
				}
				cards = append(cards, CardItem{Name: m.Name, ImgPath: mPath})
			}

			hasAny := false
			for _, card := range cards {
				if card.ImgPath != "" {
					hasAny = true
					break
				}
			}
			if !hasAny {
				continue
			}

			sRadar := prs.CreateSlide()
			tBox := sRadar.AddTextBox()
			tBox.SetOffsetX(goppt.Inch(0.4)).SetOffsetY(goppt.Inch(0.1)).SetWidth(goppt.Inch(8.5)).SetHeight(goppt.Inch(0.4))
			tp := tBox.GetActiveParagraph()
			if tp == nil {
				tp = tBox.CreateParagraph()
			}
			ttr := tp.CreateTextRun(fmt.Sprintf("Composite Reflectivity (dBZ) Comparison: %s", cs.Title))
			ttr.GetFont().SetSize(16).SetBold(true).SetColor(goppt.NewColor("FF102C57"))

			for idx, card := range cards {
				if idx >= 6 {
					break
				}
				xc := gridCoords[idx][0]
				yc := gridCoords[idx][1]

				lbl := sRadar.AddTextBox()
				lbl.SetOffsetX(xc).SetOffsetY(yc - int64(0.25*914400)).SetWidth(w).SetHeight(int64(0.25 * 914400))
				lp := lbl.GetActiveParagraph()
				if lp == nil {
					lp = lbl.CreateParagraph()
				}
				ltr := lp.CreateTextRun(card.Name)
				ltr.GetFont().SetSize(13).SetBold(true)

				if card.ImgPath != "" && isFileValid(card.ImgPath) {
					img := sRadar.CreateDrawingShape()
					img.SetPath(card.ImgPath).SetOffsetX(xc).SetOffsetY(yc).SetWidth(w).SetHeight(h)
				} else {
					ph := sRadar.AddAutoShape()
					ph.SetAutoShapeType(goppt.AutoShapeRectangle)
					ph.SetSolidFill(goppt.NewColor("FFF1F5F9"))
					ph.SetOffsetX(xc).SetOffsetY(yc).SetWidth(w).SetHeight(h)
					ph.SetText("\n\nNo Image Available")
				}
			}

			slidesData = append(slidesData, SlideData{
				Type:      "radar_case",
				Title:     fmt.Sprintf("Composite Reflectivity (dBZ) Comparison: %s", cs.Title),
				CaseTitle: cs.Title,
				Cards:     cards,
				Region:    "cases",
			})
		}
	}

	// Atomic save
	tmpPptx := fmt.Sprintf("%s.tmp.%d", outputPptx, os.Getpid())
	if err := prs.SaveToFile(tmpPptx); err != nil {
		return nil, fmt.Errorf("failed to save PPTX to %s: %w", tmpPptx, err)
	}
	if err := os.Rename(tmpPptx, outputPptx); err != nil {
		return nil, fmt.Errorf("failed to commit PPTX to %s: %w", outputPptx, err)
	}

	fmt.Printf("[DONE] Successfully created PowerPoint presentation: %s (%d slides)\n", outputPptx, len(slidesData))
	return slidesData, nil
}

func renderProfileLegend(tx *goppt.RichTextShape, models []mats.Model) {
	type textSpec struct {
		Text string
		Bold bool
		Size int
	}
	var lines []textSpec
	lines = append(lines, textSpec{Text: "Experiments:", Bold: true, Size: 11})
	for _, m := range models {
		lines = append(lines, textSpec{Text: "• " + m.Name, Bold: false, Size: 10})
	}
	lines = append(lines,
		textSpec{Text: "Line Styles:", Bold: true, Size: 11},
		textSpec{Text: "• Solid: RMSE", Bold: false, Size: 10},
		textSpec{Text: "• Dash: BIAS", Bold: false, Size: 10},
	)

	for idx, spec := range lines {
		var p *goppt.Paragraph
		if idx == 0 {
			p = tx.GetActiveParagraph()
			if p == nil {
				p = tx.CreateParagraph()
			}
		} else {
			p = tx.CreateParagraph()
		}
		tr := p.CreateTextRun(spec.Text)
		tr.GetFont().SetSize(spec.Size).SetBold(spec.Bold)
	}
}

func isFileValid(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() > 1000
}
