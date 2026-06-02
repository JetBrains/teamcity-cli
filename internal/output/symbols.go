package output

// Symbols is the glyph vocabulary for status marks and structural decoration.
// Sym() returns the Unicode set, or an ASCII-only set when ASCII is true. The
// ASCII status marks match PlainStatusIcon so both modes share one repertoire.
type Symbols struct {
	Check     string // success
	Cross     string // failure
	Neutral   string // neutral / unknown
	Skip      string // canceled / skipped
	Arrow     string // transition / "then"
	ArrowLeft string
	Bullet    string // list item
	Sep       string // inline separator (callers supply surrounding spaces)
	Pipeline  string // pipeline marker
	Recycle   string // rebuilding / shared result
	Pinned    string
	DeltaUp   string
	DeltaDown string
	Ellipsis  string
	TreeMid   string // child with following siblings
	TreeEnd   string // last child
	TreePipe  string // vertical run under TreeMid
	TreeGap   string // blank run under TreeEnd
}

var unicodeSymbols = Symbols{
	Check: "✓", Cross: "✗", Neutral: "○", Skip: "⊘",
	Arrow: "→", ArrowLeft: "←", Bullet: "•", Sep: "·",
	Pipeline: "⬡", Recycle: "⟳", Pinned: "📌",
	DeltaUp: "▲", DeltaDown: "▼", Ellipsis: "…",
	TreeMid: "├── ", TreeEnd: "└── ", TreePipe: "│   ", TreeGap: "    ",
}

var asciiSymbols = Symbols{
	Check: "+", Cross: "x", Neutral: "-", Skip: "/",
	Arrow: "->", ArrowLeft: "<-", Bullet: "*", Sep: "|",
	Pipeline: "*", Recycle: "~", Pinned: "*",
	DeltaUp: "^", DeltaDown: "v", Ellipsis: "...",
	TreeMid: "|-- ", TreeEnd: "`-- ", TreePipe: "|   ", TreeGap: "    ",
}

// Sym returns the active glyph set for the current ASCII mode.
func Sym() Symbols {
	if ASCII {
		return asciiSymbols
	}
	return unicodeSymbols
}
