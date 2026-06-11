package migrate

import "strings"

// CollectManualSetup returns the deduplicated union of ManualSetup items across results, preserving first-seen order.
func CollectManualSetup(results []*ConversionResult) []string {
	return collectUnique(results, func(r *ConversionResult) []string { return r.ManualSetup })
}

// CollectNeedsReview returns the deduplicated union of NeedsReview items across results, preserving first-seen order.
func CollectNeedsReview(results []*ConversionResult) []string {
	return collectUnique(results, func(r *ConversionResult) []string { return r.NeedsReview })
}

func collectUnique(results []*ConversionResult, items func(*ConversionResult) []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range results {
		for _, item := range items(r) {
			if !seen[item] {
				seen[item] = true
				out = append(out, item)
			}
		}
	}
	return out
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

	best := map[string]string{}
	for os, names := range byOS {
		best[os] = pickRunnerSize(names)
	}
	// Derive the label set from RunnerMap so the default and image-derived mappings can't drift apart.
	m := map[string]string{}
	for label := range RunnerMap {
		if img := best[classifyOS(label)]; img != "" {
			m[label] = img
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
