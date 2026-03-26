package output

import (
	"fmt"
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

func PrintLogo() {
	if !IsTerminal() {
		fmt.Println(Cyan("\n" + Logo))
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
					fmt.Print(" ")
				case revealed[struct{ r, c int }{r, c}]:
					fmt.Print(cyan.Render(string(ch)))
				default:
					fmt.Print(dim.Render(string(glyphs[rand.Intn(len(glyphs))])))
				}
			}
			fmt.Print("\033[K\n")
		}
	}
	fmt.Print("\033[?25l\n")
	defer fmt.Print("\033[?25h")
	moveUp := fmt.Sprintf("\033[%dA", height)
	frame := func(d time.Duration) { render(); time.Sleep(d); fmt.Print(moveUp) }
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
		fmt.Print(cyan.Render(line) + "\033[K\n")
	}
}
