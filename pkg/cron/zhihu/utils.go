package cron

import (
	"slices"

	mapset "github.com/deckarep/golang-set/v2"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

func CutSubs(subs []zhihuDB.Sub, lastCrawl string) []zhihuDB.Sub {
	index := slices.IndexFunc(subs, func(sub zhihuDB.Sub) bool {
		return sub.ID == lastCrawl
	})
	return subs[index+1:]
}

func SubsToSlice(subs []zhihuDB.Sub) (result []string) {
	for _, sub := range subs {
		result = append(result, sub.ID)
	}
	return result
}

func SliceToSubs(ids []string, subs []zhihuDB.Sub) (result []zhihuDB.Sub) {
	idSet := mapset.NewSet[string]()
	for _, i := range ids {
		idSet.Add(i)
	}

	for _, sub := range subs {
		if idSet.Contains(sub.ID) {
			result = append(result, sub)
		}
	}

	return result
}

func FilterSubs(include, exlucde, all []string) (results []string) {
	includeSet := mapset.NewSet[string]()
	excludeSet := mapset.NewSet[string]()
	allSet := mapset.NewSet[string]()

	for _, i := range include {
		includeSet.Add(i)
	}
	for _, e := range exlucde {
		excludeSet.Add(e)
	}
	for _, a := range all {
		allSet.Add(a)
	}

	var resultSet mapset.Set[string]
	if includeSet.IsEmpty() || includeSet.Contains("*") {
		resultSet = allSet.Difference(excludeSet)
	} else {
		resultSet = allSet.Intersect(includeSet)
		resultSet = resultSet.Difference(excludeSet)
	}

	return resultSet.ToSlice()
}
