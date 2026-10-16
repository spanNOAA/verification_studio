package report

import (
	"verification_studio/internal/hpc"
	"verification_studio/internal/mats"
)

// DatesConfig specifies the verification time window for surface and upper-air.
type DatesConfig struct {
	Surface  string `json:"surface"`
	UpperAir string `json:"upperair"`
}

// Config defines the full configuration for compiling reports.
type Config struct {
	Title           string          `json:"title"`
	Dates           DatesConfig     `json:"dates"`
	Models          []mats.Model    `json:"models"`
	Regions         []string        `json:"regions"`
	CrefDir         string          `json:"cref_dir"`
	ExtractedRefDir string          `json:"extracted_ref_dir,omitempty"`
	SkipCref        bool            `json:"skip_cref"`
	RadarCases      []hpc.RadarCase `json:"radar_cases"`
}

// BulletItem represents a bullet point with emphasis on the title slide.
type BulletItem struct {
	Text   string `json:"text"`
	IsBold bool   `json:"is_bold"`
}

// RowItem represents a variable row in an integrated time series slide.
type RowItem struct {
	Label string `json:"label"`
	Img1  string `json:"img1,omitempty"`
	Img2  string `json:"img2,omitempty"`
}

// CardItem represents a model or observation forecast map card.
type CardItem struct {
	Name    string `json:"name"`
	ImgPath string `json:"img_path"`
}

// SlideData holds structured metadata for 1:1 synchronization between PPTX and HTML presentations.
type SlideData struct {
	Type      string       `json:"type"` // "title", "3panel", "integrated_ts", "profile", "section_header", "radar_case"
	Title     string       `json:"title"`
	Region    string       `json:"region"`
	Subtitle  string       `json:"subtitle,omitempty"`
	CaseTitle string       `json:"case_title,omitempty"`
	Img1      string       `json:"img1,omitempty"`
	Lbl1      string       `json:"lbl1,omitempty"`
	Img2      string       `json:"img2,omitempty"`
	Lbl2      string       `json:"lbl2,omitempty"`
	Img3      string       `json:"img3,omitempty"`
	Lbl3      string       `json:"lbl3,omitempty"`
	ImgT      string       `json:"img_t,omitempty"`
	ImgW      string       `json:"img_w,omitempty"`
	ImgRH     string       `json:"img_rh,omitempty"`
	Metric    string       `json:"metric,omitempty"`
	IsBias    bool         `json:"is_bias,omitempty"`
	Col1Name  string       `json:"col1_name,omitempty"`
	Col2Name  string       `json:"col2_name,omitempty"`
	Rows      []RowItem    `json:"rows,omitempty"`
	Cards     []CardItem   `json:"cards,omitempty"`
	Bullets   []BulletItem `json:"bullets,omitempty"`
	Models    []mats.Model `json:"models,omitempty"`
	Dates     DatesConfig  `json:"dates,omitempty"`
	Regions   []string     `json:"regions,omitempty"`
}
