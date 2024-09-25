package cron

import (
	"fmt"
	"slices"
	"strconv"

	mapset "github.com/deckarep/golang-set/v2"
)

func FilterGroupIDs(include, exclude []string, all []int) (result []int, err error) {
	includeSet := mapset.NewSet[string]()
	excludeSet := mapset.NewSet[string]()
	allSet := mapset.NewSet[string]()

	for _, id := range include {
		includeSet.Add(id)
	}
	for _, id := range exclude {
		excludeSet.Add(id)
	}
	for _, id := range all {
		idStr := strconv.Itoa(id)
		allSet.Add(idStr)
	}

	includeSet.Remove("")
	excludeSet.Remove("")
	allSet.Remove("")

	var resultSet mapset.Set[string]
	if includeSet.IsEmpty() || includeSet.Contains("*") {
		resultSet = allSet.Difference(excludeSet)
	} else {
		resultSet = allSet.Intersect(includeSet)
		resultSet = resultSet.Difference(excludeSet)
	}

	result = make([]int, 0, resultSet.Cardinality())
	for id := range resultSet.Iter() {
		idInt, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("fail to convert id %s to int: %w", id, err)
		}
		result = append(result, idInt)
	}

	slices.Sort(result)
	return result, nil
}

func CutGroups(groups []int, lastCrawl int) []int {
	index := slices.Index(groups, lastCrawl)
	return groups[index+1:]
}
