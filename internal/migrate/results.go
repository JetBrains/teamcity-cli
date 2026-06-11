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

// BuildRunnerMap maps known CI runner labels to the best matching agent name; returns nil when no OS matches.
func BuildRunnerMap(imageNames []string) map[string]string {
	byOS := map[string][]string{}
	for _, name := range imageNames {
		if os := classifyOS(name); os != "" {
			byOS[os] = append(byOS[os], name)
		}
	}
	if len(byOS) == 0 {
		return nil
	}

	m := map[string]string{}
	if img := pickRunnerSize(byOS["linux"]); img != "" {
		for _, k := range []string{"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04", "ubuntu-20.04"} {
			m[k] = img
		}
	}
	if img := pickRunnerSize(byOS["mac"]); img != "" {
		for _, k := range []string{"macos-latest", "macos-15", "macos-14", "macos-13"} {
			m[k] = img
		}
	}
	if img := pickRunnerSize(byOS["windows"]); img != "" {
		for _, k := range []string{"windows-latest", "windows-2022", "windows-2019"} {
			m[k] = img
		}
	}
	return m
}

// pickRunnerSize prefers Large over Medium over XLarge over Small — closest to GHA's default 4-core runners.
func pickRunnerSize(names []string) string {
	if len(names) == 0 {
		return ""
	}
	for _, size := range []string{"large", "medium", "xlarge", "small"} {
		for _, n := range names {
			low := strings.ToLower(n)
			if size == "large" && strings.Contains(low, "xlarge") {
				continue
			}
			if strings.Contains(low, size) {
				return n
			}
		}
	}
	return names[0]
}
