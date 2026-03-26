package output

import "strings"

// StatusIcon returns a colored status icon
func StatusIcon(status, state string) string {
	if state == "running" {
		return Yellow("●")
	}
	if state == "queued" {
		return Faint("◦")
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

// StatusText returns colored status text
func StatusText(status, state string) string {
	if state == "running" {
		return Yellow("Running")
	}
	if state == "queued" {
		return Faint("Queued")
	}

	switch strings.ToUpper(status) {
	case "SUCCESS":
		return Green("Success")
	case "FAILURE":
		return Red("Failed")
	case "ERROR":
		return Red("Error")
	default:
		return status
	}
}

// PlainStatusIcon returns a plain text status icon (for --plain output)
func PlainStatusIcon(status, state string) string {
	if state == "running" {
		return "*"
	}
	if state == "queued" {
		return "o"
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

// PlainStatusText returns plain status text (for --plain output)
func PlainStatusText(status, state string) string {
	if state == "running" {
		return "running"
	}
	if state == "queued" {
		return "queued"
	}
	return strings.ToLower(status)
}
