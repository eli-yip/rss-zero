package macked

import "strings"

func isSubscribed(tilte string, names []string) bool {
	lowerTitle := strings.ToLower(tilte)
	for _, name := range names {
		if strings.HasPrefix(lowerTitle, strings.ToLower(name)) {
			return true
		}
	}
	return false
}
