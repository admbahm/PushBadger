package analyzer

import (
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"pushbadger/pkg/types"
)

// Match evaluates rules against files and returns an []AreaResult sorted by
// priority ascending then name ascending, with an "unclassified" area appended
// last for any files that matched no rule.
//
// A file may appear in multiple areas. Files within each area are sorted by
// path for deterministic output.
func Match(files []types.ChangedFile, rules []Rule) []types.AreaResult {
	// areaMap accumulates matched files per area.
	type entry struct {
		priority int
		files    []types.ChangedFile
	}
	areaMap := make(map[string]*entry)

	var unclassified []types.ChangedFile

	for _, f := range files {
		lower := strings.ToLower(f.Path)
		matched := false

		for _, r := range rules {
			if matchesAny(lower, r.Patterns) {
				matched = true
				if _, ok := areaMap[r.Area]; !ok {
					areaMap[r.Area] = &entry{priority: r.Priority}
				}
				areaMap[r.Area].files = append(areaMap[r.Area].files, f)
			}
		}

		if !matched {
			unclassified = append(unclassified, f)
		}
	}

	// Build and sort matched areas.
	areas := make([]types.AreaResult, 0, len(areaMap))
	for name, e := range areaMap {
		// Sort files within this area by path for determinism.
		sortFiles(e.files)
		areas = append(areas, types.AreaResult{
			Name:     name,
			Priority: e.priority,
			Files:    e.files,
		})
	}
	sort.Slice(areas, func(i, j int) bool {
		if areas[i].Priority != areas[j].Priority {
			return areas[i].Priority < areas[j].Priority
		}
		return areas[i].Name < areas[j].Name
	})

	// Append unclassified last.
	if len(unclassified) > 0 {
		sortFiles(unclassified)
		areas = append(areas, types.AreaResult{
			Name:     "unclassified",
			Priority: types.UnclassifiedPriority,
			Files:    unclassified,
		})
	}

	return areas
}

// matchesAny returns true if path matches any of the given doublestar patterns.
func matchesAny(path string, patterns []string) bool {
	for _, pat := range patterns {
		ok, _ := doublestar.Match(pat, path)
		if ok {
			return true
		}
	}
	return false
}

func sortFiles(files []types.ChangedFile) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
}
