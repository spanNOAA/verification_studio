package mats

import (
	"sort"
	"strconv"
	"strings"
)

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

// Default baseline forecast leads for each meteorological element.
var (
	DefaultSurfaceLeads  = []string{"1", "3", "12", "24"}
	DefaultUpperAirLeads = []string{"1", "3", "6", "12"}
	DefaultRadarLeads    = []string{"1", "3"}
	DefaultCeilVisLeads  = []string{"1", "3"}
)

// SortLeadHours cleans, deduplicates, and sorts forecast lead strings in ascending numerical order.
func SortLeadHours(leads []string) []string {
	if len(leads) == 0 {
		return nil
	}
	type item struct {
		val int
		raw string
	}
	var items []item
	seen := make(map[string]bool)
	for _, l := range leads {
		trimmed := strings.TrimSpace(strings.TrimSuffix(strings.ToLower(l), "h"))
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		n, err := strconv.Atoi(trimmed)
		if err != nil {
			continue
		}
		items = append(items, item{val: n, raw: strconv.Itoa(n)})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].val < items[j].val
	})
	res := make([]string, len(items))
	for i, it := range items {
		res[i] = it.raw
	}
	return res
}

// ResolveLeadHours resolves lead hours against defaults.
// If configured is nil, it falls back to defaults.
// If configured is explicitly provided as empty ([]string{}), it returns empty slice indicating the component is skipped.
func ResolveLeadHours(configured []string, defaults []string) []string {
	if configured == nil {
		return defaults
	}
	if len(configured) == 0 {
		return []string{}
	}
	sorted := SortLeadHours(configured)
	if sorted == nil {
		return []string{}
	}
	return sorted
}
