package output

import "golang.org/x/sys/windows"

// cpUTF8 is the Windows console code page for UTF-8.
const cpUTF8 = 65001

// ConsoleSupportsUTF8 reports whether the active console can render UTF-8.
// On Windows this is true only when the output code page is UTF-8 (65001);
// legacy code pages (437, 866, …) mangle multi-byte glyphs, so we fall back to
// ASCII. A failed query is treated as capable to avoid false downgrades.
func ConsoleSupportsUTF8() bool {
	cp, err := windows.GetConsoleOutputCP()
	if err != nil {
		return true
	}
	return cp == cpUTF8
}
