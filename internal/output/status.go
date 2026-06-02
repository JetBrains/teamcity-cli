package output

import "strings"

func isCanceled(status, statusText string) bool {
	return strings.EqualFold(status, "UNKNOWN") && strings.HasPrefix(strings.ToLower(statusText), "canceled")
}

// StatusIcon returns a colored status icon. In ASCII mode the glyph degrades to
// its PlainStatusIcon equivalent while keeping the color.
func StatusIcon(status, state string, statusText ...string) string {
	glyph, color := statusGlyph(status, state, statusText...)
	if ASCII {
		glyph = PlainStatusIcon(status, state, statusText...)
	}
	return color(glyph)
}

// statusGlyph returns the Unicode icon and its color function for a status.
func statusGlyph(status, state string, statusText ...string) (string, func(...any) string) {
	switch {
	case state == "running":
		return "●", Yellow
	case state == "queued":
		return "◦", Faint
	case len(statusText) > 0 && isCanceled(status, statusText[0]):
		return "⊘", Faint
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return "✓", Green
	case "FAILURE", "ERROR":
		return "✗", Red
	case "UNKNOWN":
		return "?", Yellow
	default:
		return "○", Faint
	}
}

// StatusText returns colored status text.
func StatusText(status, state string, apiStatusText ...string) string {
	if state == "running" {
		return Yellow("Running")
	}
	if state == "queued" {
		return Faint("Queued")
	}

	if len(apiStatusText) > 0 && isCanceled(status, apiStatusText[0]) {
		return Faint("Canceled")
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return Green("Success")
	case "FAILURE":
		return Red("Failed")
	case "ERROR":
		return Red("Error")
	case "UNKNOWN":
		return Yellow("Unknown")
	default:
		return status
	}
}

// PlainStatusIcon returns a plain text status icon (for --plain output).
func PlainStatusIcon(status, state string, statusText ...string) string {
	if state == "running" {
		return "*"
	}
	if state == "queued" {
		return "o"
	}

	if len(statusText) > 0 && isCanceled(status, statusText[0]) {
		return "/"
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return "+"
	case "FAILURE", "ERROR":
		return "x"
	case "UNKNOWN":
		return "?"
	default:
		return "-"
	}
}

// PlainStatusText returns plain status text (for --plain output).
func PlainStatusText(status, state string, apiStatusText ...string) string {
	if state == "running" {
		return "running"
	}
	if state == "queued" {
		return "queued"
	}
	if len(apiStatusText) > 0 && isCanceled(status, apiStatusText[0]) {
		return "canceled"
	}
	return strings.ToLower(status)
}
