package endoflife

import (
	"fmt"
	"sort"
	"time"
)

// versionInfo contains the version and release date of the latest Mattermost version
type versionInfo struct {
	version     version
	releaseDate time.Time
	lts         bool
}

// version contains the major, minor, and patch version numbers
type version struct {
	major int
	minor int
	Patch int
}

func ParseCycles(cycles []cycle) (versionInfoList []versionInfo, err error) {
	if len(cycles) == 0 {
		return nil, fmt.Errorf("found no cycles")
	}

	cycles, err = filterCycles(cycles)
	if err != nil {
		return nil, fmt.Errorf("fail to filter cycles: %w", err)
	}

	versionInfoList = make([]versionInfo, 0, len(cycles))

	for _, c := range cycles {
		v, err := versionStringToVersion(c.Latest)
		if err != nil {
			return nil, fmt.Errorf("fail to parse version: %w", err)
		}

		t, err := time.Parse("2006-01-02", c.LatestReleaseDate)
		if err != nil {
			return nil, fmt.Errorf("fail to parse release date: %w", err)
		}

		versionInfoList = append(versionInfoList, versionInfo{
			version:     v,
			releaseDate: t,
			lts:         c.Lts,
		})
	}

	sortVersionInfoList(versionInfoList)

	return versionInfoList, nil
}

func getLatestVersion(cycles []cycle) (version string, err error) {
	if len(cycles) == 0 {
		return "", fmt.Errorf("found no cycles")
	}

	return cycles[0].Latest, nil
}

func filterCycles(cycles []cycle) (filteredCycles []cycle, err error) {
	latestVersion, err := getLatestVersion(cycles)
	if err != nil {
		return nil, fmt.Errorf("fail to get latest version: %w", err)
	}

	for _, c := range cycles {
		if checkEOL(c.Eol) {
			continue
		}

		if c.Latest == latestVersion {
			filteredCycles = append(filteredCycles, c)
		}
		if c.Lts {
			filteredCycles = append(filteredCycles, c)
		}
	}
	return filteredCycles, nil
}

func checkEOL(eol any) bool {
	if eolBool, ok := eol.(bool); ok {
		return eolBool
	}

	if eolStr, ok := eol.(string); ok {
		if eolStr != "" {
			eolTime, err := parseTime(eolStr)
			if err != nil {
				return false
			}
			return eolTime.Before(time.Now())
		}
	}

	return false
}

func versionStringToVersion(versionString string) (v version, err error) {
	n, err := fmt.Sscanf(versionString, "%d.%d.%d", &v.major, &v.minor, &v.Patch)
	if err == nil && n == 3 {
		return v, nil
	}

	n, err = fmt.Sscanf(versionString, "%d.%d", &v.major, &v.minor)
	if err == nil && n == 2 {
		v.Patch = 0
		return v, nil
	}

	if err != nil {
		return version{}, fmt.Errorf("fail to parse version string: %w", err)
	}
	return version{}, fmt.Errorf("fail to parse version string: %s", versionString)
}

func versionToVersionString(version version) string {
	return fmt.Sprintf("%d.%d.%d", version.major, version.minor, version.Patch)
}

func sortVersionInfoList(versionInfoList []versionInfo) {
	sort.Slice(versionInfoList, func(i, j int) bool {
		return compareVersion(versionInfoList[i].version, versionInfoList[j].version) > 0
	})
}

func compareVersion(a version, b version) int {
	if a.major != b.major {
		return a.major - b.major
	}
	if a.minor != b.minor {
		return a.minor - b.minor
	}
	return a.Patch - b.Patch
}

func parseTime(timeString string) (time.Time, error) {
	return time.Parse("2006-01-02", timeString)
}
