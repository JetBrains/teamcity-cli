package migrate

import "strings"

// CollectManualSetup returns the deduplicated union of ManualSetup items across results, preserving first-seen order.
func CollectManualSetup(results []*ConversionResult) []string {
	seen := map[string]bool{}
	items := []string{}
	for _, r := range results {
		for _, item := range r.ManualSetup {
			if !seen[item] {
				seen[item] = true
				items = append(items, item)
			}
		}
	}
	return items
}

// HasValidationErrors reports whether any result has a non-empty ValidationError.
func HasValidationErrors(results []*ConversionResult) bool {
	for _, r := range results {
		if r.ValidationError != "" {
			return true
		}
	}
	return false
}

// BuildRunnerMap maps known CI runner labels to the first matching cloud image name; returns nil when no OS matches.
func BuildRunnerMap(imageNames []string) map[string]string {
	byOS := map[string]string{}
	for _, name := range imageNames {
		n := strings.ToLower(name)
		switch {
		case strings.Contains(n, "ubuntu") || strings.Contains(n, "linux"):
			if _, ok := byOS["linux"]; !ok {
				byOS["linux"] = name
			}
		case strings.Contains(n, "macos") || strings.Contains(n, "mac"):
			if _, ok := byOS["mac"]; !ok {
				byOS["mac"] = name
			}
		case strings.Contains(n, "windows"):
			if _, ok := byOS["windows"]; !ok {
				byOS["windows"] = name
			}
		}
	}
	if len(byOS) == 0 {
		return nil
	}

	m := map[string]string{}
	if img, ok := byOS["linux"]; ok {
		for _, k := range []string{"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04", "ubuntu-20.04"} {
			m[k] = img
		}
	}
	if img, ok := byOS["mac"]; ok {
		for _, k := range []string{"macos-latest", "macos-15", "macos-14", "macos-13"} {
			m[k] = img
		}
	}
	if img, ok := byOS["windows"]; ok {
		for _, k := range []string{"windows-latest", "windows-2022", "windows-2019"} {
			m[k] = img
		}
	}
	return m
}
