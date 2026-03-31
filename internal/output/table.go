package output

import (
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mattn/go-runewidth"
)

// renderTable renders a formatted table string with proper Unicode/ANSI handling.
func renderTable(headers []string, rows [][]string) string {
	noBorder := lipgloss.Border{}
	headerStyle := lipgloss.NewStyle().Faint(true)
	cellStyle := lipgloss.NewStyle()

	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(noBorder).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			padding := 2
			if col == len(headers)-1 {
				padding = 0
			}
			if row == table.HeaderRow {
				return headerStyle.PaddingRight(padding)
			}
			return cellStyle.PaddingRight(padding)
		})

	return strings.TrimSpace(t.Render())
}

// renderPlainTable renders tab-separated output for scripting (works with cut -f, awk).
func renderPlainTable(headers []string, rows [][]string, noHeader bool) string {
	for i, row := range rows {
		for j, cell := range row {
			rows[i][j] = stripansi.Strip(cell)
		}
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = runewidth.StringWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				if w := runewidth.StringWidth(cell); w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}

	padRow := func(cells []string) string {
		padded := make([]string, len(cells))
		for i, cell := range cells {
			if i < len(colWidths) {
				padded[i] = runewidth.FillRight(cell, colWidths[i])
			} else {
				padded[i] = cell
			}
		}
		return strings.Join(padded, "\t")
	}

	var b strings.Builder
	if !noHeader {
		b.WriteString(padRow(headers))
		b.WriteByte('\n')
	}
	for _, row := range rows {
		b.WriteString(padRow(row))
		b.WriteByte('\n')
	}
	return b.String()
}

// AutoSizeColumns truncates flexible columns in-place to fit the terminal width.
// Fixed columns keep their natural width; the remaining space goes to flex columns.
func AutoSizeColumns(headers []string, rows [][]string, padding int, flexCols ...int) {
	if len(rows) == 0 || len(flexCols) == 0 {
		return
	}

	maxW := measureColumnWidths(headers, rows)
	n := len(maxW)

	var flex []int
	isFlex := make([]bool, n)
	for _, c := range flexCols {
		if c >= 0 && c < n && !isFlex[c] {
			flex = append(flex, c)
			isFlex[c] = true
		}
	}
	if len(flex) == 0 {
		return
	}

	fixed := padding * (n - 1)
	for i, w := range maxW {
		if !isFlex[i] {
			fixed += w
		}
	}
	budget := max(TerminalWidth()-fixed, 8*len(flex))

	needs := make([]int, len(flex))
	for i, c := range flex {
		needs[i] = maxW[c]
	}
	alloc := distributeSpace(budget, needs)

	for _, row := range rows {
		for i, c := range flex {
			if c < len(row) {
				row[c] = Truncate(row[c], alloc[i])
			}
		}
	}
}

// measureColumnWidths returns the max display width per column (ANSI-aware).
func measureColumnWidths(headers []string, rows [][]string) []int {
	n := len(headers)
	for _, row := range rows {
		n = max(n, len(row))
	}
	widths := make([]int, n)
	for i, h := range headers {
		widths[i] = runewidth.StringWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := lipgloss.Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

// distributeSpace divides budget among columns. Columns that fit get their full
// width; the rest is split proportionally among those that overflow.
func distributeSpace(budget int, needs []int) []int {
	alloc := make([]int, len(needs))
	remaining := budget
	settled := make([]bool, len(needs))

	for {
		unsettled := 0
		for i := range needs {
			if !settled[i] {
				unsettled++
			}
		}
		if unsettled == 0 {
			break
		}

		fair := remaining / unsettled
		changed := false
		for i, need := range needs {
			if !settled[i] && need <= fair {
				alloc[i] = need
				remaining -= need
				settled[i] = true
				changed = true
			}
		}

		if !changed {
			totalNeed := 0
			for i, need := range needs {
				if !settled[i] {
					totalNeed += need
				}
			}
			for i, need := range needs {
				if !settled[i] {
					alloc[i] = remaining * need / totalNeed
				}
			}
			break
		}
	}

	return alloc
}

// Truncate truncates a string to maxLen display width, adding "..." if truncated
// Properly handles unicode and wide characters
func Truncate(s string, maxLen int) string {
	if runewidth.StringWidth(s) <= maxLen {
		return s
	}
	return runewidth.Truncate(s, maxLen, "...")
}
