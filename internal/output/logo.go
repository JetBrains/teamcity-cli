package output

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const Logo = `████████╗ ██████╗
╚══██╔══╝██╔════╝
   ██║   ██║
   ██║   ██║
   ██║   ╚██████╗
   ╚═╝    ╚═════╝`

func PrintLogo(w io.Writer) {
	if !IsTerminal() {
		_, _ = fmt.Fprintln(w, Cyan("\n"+Logo))
		return
	}
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#006666"))
	lines := strings.Split(Logo, "\n")
	height := len(lines)
	var chars []struct{ r, c int }
	for r, line := range lines {
		for c, ch := range []rune(line) {
			if ch != ' ' {
				chars = append(chars, struct{ r, c int }{r, c})
			}
		}
	}
	rand.Shuffle(len(chars), func(i, j int) { chars[i], chars[j] = chars[j], chars[i] })
	revealed := make(map[struct{ r, c int }]bool)
	glyphs := []rune("01アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン@#$%&*<>[]{}=+-~")
	render := func() {
		for r, line := range lines {
			for c, ch := range []rune(line) {
				switch {
				case ch == ' ':
					_, _ = fmt.Fprint(w, " ")
				case revealed[struct{ r, c int }{r, c}]:
					_, _ = fmt.Fprint(w, cyan.Render(string(ch)))
				default:
					_, _ = fmt.Fprint(w, dim.Render(string(glyphs[rand.Intn(len(glyphs))])))
				}
			}
			_, _ = fmt.Fprint(w, "\033[K\n")
		}
	}
	_, _ = fmt.Fprint(w, "\033[?25l\n")
	defer func() { _, _ = fmt.Fprint(w, "\033[?25h") }()
	moveUp := fmt.Sprintf("\033[%dA", height)
	frame := func(d time.Duration) { render(); time.Sleep(d); _, _ = fmt.Fprint(w, moveUp) }
	for range 10 {
		frame(50 * time.Millisecond)
	}
	perFrame := max(len(chars)/15, 2)
	for i := 0; i < len(chars); i += perFrame {
		for j := i; j < min(i+perFrame, len(chars)); j++ {
			revealed[chars[j]] = true
		}
		frame(40 * time.Millisecond)
	}
	for range 6 {
		frame(50 * time.Millisecond)
	}
	for _, line := range lines {
		_, _ = fmt.Fprint(w, cyan.Render(line)+"\033[K\n")
	}
}
