package output

import (
	"encoding/base64"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
)

// Program states from https://www.superlogical.com/rex/docs/build/program-status.
const (
	ProgramWorking = "working"
	ProgramDone    = "done"
	ProgramError   = "error"
	ProgramIdle    = "idle"
	ProgramClear   = "clear"
)

// ProgramStatus reports OSC 7501 status to terminal stderr, including in quiet mode.
func (p *Printer) ProgramStatus(state, message string) {
	p.setProgramStatus(programStatus(state, "", message))
}

// ProgramWait reports a human-input wait and returns a function restoring the previous status.
func (p *Printer) ProgramWait(kind, message string) func() {
	previous := p.setProgramStatus(programStatus("blocked", kind, message))
	return func() { p.setProgramStatus(previous) }
}

func (p *Printer) setProgramStatus(sequence string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	previous := p.programStatus
	if previous == "" {
		previous = programStatus(ProgramIdle, "", "")
	}
	p.programStatus = sequence
	if p.SuppressProgramStatus || os.Getenv("TERM") == "dumb" || os.Getenv("CI") != "" || os.Getenv("TEAMCITY_VERSION") != "" {
		return previous
	}
	w := p.ErrOut
	if s, ok := w.(stopWriter); ok {
		w = s.w // Status updates must not stop the activity spinner.
	}
	f, ok := w.(*os.File)
	if ok && term.IsTerminal(int(f.Fd())) {
		_, _ = io.WriteString(w, sequence)
	}
	return previous
}

func programStatus(state, kind, message string) string {
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, message)
	if len(message) > 2048 {
		message = strings.ToValidUTF8(message[:2048], "")
	}
	sequence := "\x1b]7501;state=" + state + ":app=teamcity"
	if kind != "" {
		sequence += ":kind=" + kind
	}
	if message != "" {
		sequence += ":msg=" + base64.StdEncoding.EncodeToString([]byte(message))
	}
	return sequence + "\x1b\\"
}
