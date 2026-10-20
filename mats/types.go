package mats

// Model represents an evaluated NWP forecast model experiment.
type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Path    string `json:"path,omitempty"`
	Cluster string `json:"cluster,omitempty"`
}

// DefaultPalette provides standard high-contrast distinct colors for curves (Modern Tech).
var DefaultPalette = []string{
	"#1d4ed8", // Deep Tech Blue
	"#ef4444", // Bright Red
	"#059669", // Emerald Green
	"#f59e0b", // Amber Gold
	"#7e22ce", // Deep Violet
	"#06b6d4", // Electric Cyan
	"#1e293b", // Slate Charcoal
	"#e11d48", // Rose Red
}

// RegionNameMap maps internal region identifiers to human-readable names.
var RegionNameMap = map[string]string{
	"all":  "Full Domain (All HRRR)",
	"east": "Eastern Region",
	"west": "Western Region (Complex Terrain)",
}

// SurfaceRegionLabels maps region IDs to MATS predefined dropdown values for surface.
var SurfaceRegionLabels = map[string]string{
	"all":  "All HRRR domain",
	"east": "Eastern HRRR domain",
	"west": "Western HRRR domain",
}

// CeilVisRegionLabels maps region IDs to MATS predefined dropdown values for ceiling & visibility.
var CeilVisRegionLabels = map[string]string{
	"all":  "All HRRR domain",
	"east": "Eastern HRRR domain",
	"west": "Western HRRR domain",
}

// UpperAirRegionLabels maps region IDs to MATS predefined dropdown values for upper air.
var UpperAirRegionLabels = map[string]string{
	"all":  "HRRR domain",
	"east": "East of 109W",
	"west": "West of 109W",
}

// RadarRegionLabels maps region IDs to MATS predefined dropdown values for radar.
var RadarRegionLabels = map[string]string{
	"all":  "Continental US",
	"east": "Eastern US (lon <= 100W)",
	"west": "Western US (lon >= 100W)",
}

// CheckOptions configures pre-checks for model availability in MATS databases.
type CheckOptions struct {
	Category string
	Variable string
	Database string
}
