package macked

import (
	"slices"
	"strings"
)

// appIndex returns the index of the app in the names slice, -1 if not found.
func appIndex(title string, names []string) int {
	return slices.IndexFunc(names, func(name string) bool { return strings.HasPrefix(strings.ToLower(title), strings.ToLower(name)) })
}
