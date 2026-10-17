package hpc

import (
	"regexp"
)

// ProtocolVersion defines the enforced read-only communication protocol version.
const ProtocolVersion = "RO-v1"

// MaxDownloadBytes defines the maximum allowable download payload (50 MB).
const MaxDownloadBytes = 52428800

// ReadOnlyOp defines permitted structured operations in the RO-v1 specification.
type ReadOnlyOp string

const (
	OpList     ReadOnlyOp = "list"
	OpStat     ReadOnlyOp = "stat"
	OpDownload ReadOnlyOp = "download"
	OpPing     ReadOnlyOp = "ping"
	OpStatus   ReadOnlyOp = "status"
	OpStop     ReadOnlyOp = "stop"
)

// AllowedRemoteOps defines operations permitted on the remote cluster filesystems.
var AllowedRemoteOps = map[ReadOnlyOp]bool{
	OpList:     true,
	OpStat:     true,
	OpDownload: true,
}

// AllowedControlOps defines daemon lifecycle and health control operations.
var AllowedControlOps = map[ReadOnlyOp]bool{
	OpPing:   true,
	OpStatus: true,
	OpStop:   true,
}

// RadarCase defines a severe convective weather case study for synoptic radar comparison.
type RadarCase struct {
	Index string `json:"index"`
	Date  string `json:"date"`
	Cycle string `json:"cycle"`
	Fhr   int    `json:"fhr"`
	Title string `json:"title"`
}

// ClusterInfo describes connection targets and regex rules for a supported supercomputer.
type ClusterInfo struct {
	SSHHost         string
	CanonicalHost   string
	AliasCandidates []string
	Description     string
	TimeoutInit     int
	PromptRegex     *regexp.Regexp
}

// Clusters maps all supported NOAA RDHPCS and partner supercomputers.
var Clusters = map[string]ClusterInfo{
	"gaea-c5": {
		SSHHost:         "Gaeac5",
		CanonicalHost:   "gaea-c5.fairmont.rdhpcs.noaa.gov",
		AliasCandidates: []string{"Gaeac5", "gaea-c5", "gaeac5", "c5", "Gaea", "gaea"},
		Description:     "NOAA RDHPCS Gaea C5 (gaea51~56, /gpfs/f5)",
		TimeoutInit:     60,
		PromptRegex:     regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@gaea\d+:.*?>|gaea\d+:.*?>|\[[a-zA-Z0-9_\.\-]+@gaea\d+.*?\][$#>]|:~>)`),
	},
	"gaea-c6": {
		SSHHost:         "Gaeac6",
		CanonicalHost:   "gaea-c6.fairmont.rdhpcs.noaa.gov",
		AliasCandidates: []string{"Gaeac6", "gaea-c6", "gaeac6", "c6"},
		Description:     "NOAA RDHPCS Gaea C6 (gaea61~66, /gpfs/f6)",
		TimeoutInit:     60,
		PromptRegex:     regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@gaea\d+:.*?>|gaea\d+:.*?>|\[[a-zA-Z0-9_\.\-]+@gaea\d+.*?\][$#>]|:~>)`),
	},
	"gaea-c7": {
		SSHHost:         "Gaeac7",
		CanonicalHost:   "gaea-c7.fairmont.rdhpcs.noaa.gov",
		AliasCandidates: []string{"Gaeac7", "gaea-c7", "gaeac7", "c7"},
		Description:     "NOAA RDHPCS Gaea C7 (gaea71~76, /gpfs/f7)",
		TimeoutInit:     90,
		PromptRegex:     regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@gaea\d+:.*?>|gaea\d+:.*?>|\[[a-zA-Z0-9_\.\-]+@gaea\d+.*?\][$#>]|:~>)`),
	},
	"hera": {
		SSHHost:         "Hera",
		CanonicalHost:   "hera-mfa.fairmont.rdhpcs.noaa.gov",
		AliasCandidates: []string{"Hera", "hera"},
		Description:     "NOAA RDHPCS Hera",
		TimeoutInit:     50,
		PromptRegex:     regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@hfe\d+|hfe\d+:[^$#]*[$#]|\[[a-zA-Z0-9_\.\-]+@hfe\d+.*?\][$#>]|:~>)`),
	},
	"ursa": {
		SSHHost:         "Ursa",
		CanonicalHost:   "ursa-mfa.fairmont.rdhpcs.noaa.gov",
		AliasCandidates: []string{"Ursa", "ursa"},
		Description:     "NOAA RDHPCS Ursa",
		TimeoutInit:     50,
		PromptRegex:     regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@ufe\d+|ufe\d+:[^$#]*[$#]|\[[a-zA-Z0-9_\.\-]+@ufe\d+.*?\][$#>]|:~>)`),
	},
	"orion": {
		SSHHost:         "Orion",
		CanonicalHost:   "orion-login.hpc.msstate.edu",
		AliasCandidates: []string{"Orion", "orion"},
		Description:     "MSU Orion",
		TimeoutInit:     35,
		PromptRegex:     regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@Orion|orion-login|\[[a-zA-Z0-9_\.\-]+@Orion.*?\][$#>]|:~>)`),
	},
	"hercules": {
		SSHHost:         "Hercules",
		CanonicalHost:   "hercules-login.hpc.msstate.edu",
		AliasCandidates: []string{"Hercules", "hercules"},
		Description:     "MSU Hercules",
		TimeoutInit:     35,
		PromptRegex:     regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@hercules|hercules-login|\[[a-zA-Z0-9_\.\-]+@hercules.*?\][$#>]|:~>)`),
	},
}

// ClusterAliases maps user-provided shorthands to canonical cluster names.
var ClusterAliases = map[string]string{
	"c5":       "gaea-c5",
	"c6":       "gaea-c6",
	"c7":       "gaea-c7",
	"gaeac5":   "gaea-c5",
	"gaeac6":   "gaea-c6",
	"gaeac7":   "gaea-c7",
	"gaea-c5":  "gaea-c5",
	"gaea-c6":  "gaea-c6",
	"gaea-c7":  "gaea-c7",
	"hera":     "hera",
	"ursa":     "ursa",
	"orion":    "orion",
	"hercules": "hercules",
}

// GenericPromptRegex serves as a universal shell prompt detection pattern.
var GenericPromptRegex = regexp.MustCompile(`([a-zA-Z0-9_\.\-]+@[a-zA-Z0-9_\.\-]+[:\s][^$#>]*[$#>]\s*$|:~[>#]\s*$|\[[a-zA-Z0-9_\.\-]+@.+?\]\s*[$#>]\s*$)`)

// Request represents a structured JSON-RPC request to the HPC daemon.
type Request struct {
	Op       ReadOnlyOp `json:"op"`
	Path     string     `json:"path,omitempty"`
	Protocol string     `json:"protocol"`
}
