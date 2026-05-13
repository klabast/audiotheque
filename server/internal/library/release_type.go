package library

import "strings"

// detectReleaseType infers an album version label from a folder path. Used to
// give the UI a quick way to distinguish e.g. a hi-res rip from a remaster.
// Order matters: "remaster" before "hi-res" because "remaster (hi-res)"
// folders should surface as remasters.
//
// Ported from v1 (server/internal/library/sql_repository.go: detectReleaseType).
func detectReleaseType(folderPath string) string {
	lower := strings.ToLower(folderPath)
	switch {
	case strings.Contains(lower, "remaster"):
		return "remaster"
	case strings.Contains(lower, "deluxe"):
		return "deluxe"
	case strings.Contains(lower, "special edition"), strings.Contains(lower, "special_edition"):
		return "special"
	case strings.Contains(lower, "anniversary"):
		return "anniversary"
	case strings.Contains(lower, "expanded"):
		return "expanded"
	case strings.Contains(lower, "bonus"):
		return "bonus"
	case strings.Contains(lower, "hi-res"), strings.Contains(lower, "hires"), strings.Contains(lower, "24bit"):
		return "hi-res"
	case strings.Contains(lower, "live"):
		return "live"
	case strings.Contains(lower, "demo"):
		return "demo"
	case strings.Contains(lower, "compilation"), strings.Contains(lower, "greatest hits"):
		return "compilation"
	}
	return "original"
}
