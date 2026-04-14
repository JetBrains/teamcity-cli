package output

import "strings"

func isCanceled(status, statusText string) bool {
	return strings.EqualFold(status, "UNKNOWN") && strings.HasPrefix(strings.ToLower(statusText), "canceled")
}

// StatusIcon returns a colored status icon.
func StatusIcon(status, state string, statusText ...string) string {
	if state == "running" {
		return Yellow("●")
	}
	if state == "queued" {
		return Faint("◦")
	}

	if len(statusText) > 0 && isCanceled(status, statusText[0]) {
		return Faint("⊘")
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return Green("✓")
	case "FAILURE", "ERROR":
		return Red("✗")
	case "UNKNOWN":
		return Yellow("?")
	default:
		return Faint("○")
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
