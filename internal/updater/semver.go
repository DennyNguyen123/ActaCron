package updater

import (
	"strconv"
	"strings"
)

// IsNewerVersion returns true if latest is strictly newer than current.
func IsNewerVersion(current, latest string) bool {
	cleanCur := strings.TrimPrefix(strings.TrimSpace(current), "v")
	cleanLat := strings.TrimPrefix(strings.TrimSpace(latest), "v")

	if cleanCur == "" || cleanCur == "dev" {
		return cleanLat != "" && cleanLat != "dev"
	}
	if cleanLat == "" || cleanLat == "dev" {
		return false
	}

	curParts := parseVersionParts(cleanCur)
	latParts := parseVersionParts(cleanLat)

	maxLen := len(curParts)
	if len(latParts) > maxLen {
		maxLen = len(latParts)
	}

	for i := 0; i < maxLen; i++ {
		curVal := 0
		if i < len(curParts) {
			curVal = curParts[i]
		}
		latVal := 0
		if i < len(latParts) {
			latVal = latParts[i]
		}

		if latVal > curVal {
			return true
		} else if latVal < curVal {
			return false
		}
	}

	return false
}

func parseVersionParts(v string) []int {
	// Cut off pre-release or build metadata (e.g. 1.0.0-rc1 -> 1.0.0)
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	rawParts := strings.Split(v, ".")
	parts := make([]int, 0, len(rawParts))
	for _, p := range rawParts {
		if n, err := strconv.Atoi(p); err == nil {
			parts = append(parts, n)
		} else {
			parts = append(parts, 0)
		}
	}
	return parts
}
