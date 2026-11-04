package onboard

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const bannerWidth = 18

const bannerLeft = 2

var bannerFront = strings.Split(output.Logo, "\n")

var bannerBack = []string{
	"██████╗  ██████╗ ",
	"╚════██╗██╔═████╗",
	" █████╔╝██║██╔██║",
	"██╔═══╝ ████╔╝██║",
	"███████╗╚██████╔╝",
	"╚══════╝ ╚═════╝ ",
}

var flipSteps = []int{18, 15, 12, 9, 6, 3, 1, 0, 1, 3, 6, 9, 12, 15, 18}

const flipFrame = 28 * time.Millisecond

var (
	bannerTitle = lipgloss.NewStyle().Bold(true) // no fixed color: must stay readable on light themes
	bannerDim   = lipgloss.NewStyle().Faint(true)
	bannerBold  = lipgloss.NewStyle().Bold(true)
)

var gradientStops = [][3]float64{{0x07, 0xc3, 0xf2}, {0x3b, 0x6c, 0xff}, {0x90, 0x5c, 0xfb}}

func gradientColor(col int) lipgloss.Color {
	t := float64(col) / float64(bannerWidth-1) * float64(len(gradientStops)-1)
	i := min(int(t), len(gradientStops)-2)
	f := t - float64(i)
	a, b := gradientStops[i], gradientStops[i+1]
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x",
		int(a[0]+(b[0]-a[0])*f), int(a[1]+(b[1]-a[1])*f), int(a[2]+(b[2]-a[2])*f)))
}

// w<=2 draws a spine so the edge-on glyph doesn't vanish.
func squash(art []string, w int) []string {
	out := make([]string, len(art))
	for i, line := range art {
		ch := []rune(line + strings.Repeat(" ", bannerWidth))[:bannerWidth]
		var b strings.Builder
		left := (bannerWidth - w) / 2
		b.WriteString(strings.Repeat(" ", left))
		for c := range w {
			r := ch[min(len(ch)-1, c*bannerWidth/w)]
			if w <= 2 && r != ' ' {
				r = '│'
			}
			b.WriteRune(r)
		}
		b.WriteString(strings.Repeat(" ", bannerWidth-left-w))
		out[i] = b.String()
	}
	return out
}

func renderBanner(art []string, side []string) string {
	var b strings.Builder
	for i, line := range art {
		b.WriteString(strings.Repeat(" ", bannerLeft))
		for c, r := range []rune(line) {
			if r == ' ' {
				b.WriteByte(' ')
				continue
			}
			b.WriteString(bannerBold.Foreground(gradientColor(c)).Render(string(r)))
		}
		if i < len(side) && side[i] != "" {
			b.WriteString("   " + side[i])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func sideFront() []string {
	return []string{"", bannerTitle.Render("TeamCity CLI  v" + version.String()), bannerDim.Render("Onboarding · from zero to green build hero")}
}

func sideBack() []string {
	return []string{"", bannerTitle.Render("YEARS OF RELIABLE CI"), bannerDim.Render("since 2006 · still making you hero")}
}

type bannerTick struct{}

// Intro frame counts and delays mirror output.PrintLogo.
const (
	introNoise = 10
	introHold  = 6
)

var introRank, introReveal, introTotal = func() (map[[2]int]int, int, int) {
	var cells [][2]int
	for r, line := range bannerFront {
		for c, ch := range []rune(line) {
			if ch != ' ' {
				cells = append(cells, [2]int{r, c})
			}
		}
	}
	rand.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })
	rank := make(map[[2]int]int, len(cells))
	for i, c := range cells {
		rank[c] = i
	}
	perFrame := max(len(cells)/15, 2)
	revealFrames := (len(cells) + perFrame - 1) / perFrame
	return rank, perFrame, introNoise + revealFrames + introHold
}()

var introDim = lipgloss.NewStyle().Foreground(lipgloss.Color("#006666"))

func introDelay(elapsed int) time.Duration {
	if elapsed < introNoise || elapsed >= introTotal-introHold {
		return 50 * time.Millisecond
	}
	return 40 * time.Millisecond
}

func renderIntro(elapsed int) string {
	revealed := max(0, elapsed-introNoise+1) * introReveal
	var b strings.Builder
	b.WriteString("\n")
	for r, line := range bannerFront {
		b.WriteString(strings.Repeat(" ", bannerLeft))
		for c, ch := range []rune(line) {
			switch rank, ok := introRank[[2]int{r, c}]; {
			case !ok:
				b.WriteByte(' ')
			case rank < revealed:
				b.WriteString(bannerBold.Foreground(gradientColor(c)).Render(string(ch)))
			default:
				b.WriteString(introDim.Render(string(output.LogoGlyphs[rand.Intn(len(output.LogoGlyphs))])))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// ponytail: inline bubbletea doesn't know its screen row, so hover matches the glyph columns on any row.
type bannerModel struct {
	intro   int
	back    bool
	step    int // index into flipSteps; len(flipSteps)-1 when at rest
	hover   bool
	done    bool
	aborted bool
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return bannerTick{} })
}

func (m bannerModel) Init() tea.Cmd {
	if m.intro > 0 {
		return tick(introDelay(0))
	}
	return nil
}

func (m bannerModel) atRest() bool { return m.step >= len(flipSteps)-1 }

func (m bannerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.done, m.aborted = true, msg.Type == tea.KeyCtrlC
		return m, tea.Quit
	case tea.MouseMsg:
		over := msg.X >= bannerLeft && msg.X < bannerLeft+bannerWidth
		if over == m.hover {
			return m, nil
		}
		m.hover = over
		if m.intro > 0 {
			return m, nil // the intro's tick chain picks the hover up when it ends
		}
		if m.atRest() {
			m.step = 0
			return m, tick(flipFrame)
		}
		// Mid-flip: play the remaining frames backwards from the same width.
		m.step = len(flipSteps) - 1 - m.step
		return m, nil
	case bannerTick:
		if m.intro > 0 {
			m.intro--
			if m.intro > 0 {
				return m, tick(introDelay(introTotal - m.intro))
			}
			if m.hover {
				m.step = 0
				return m, tick(flipFrame)
			}
			return m, nil
		}
		m.step++
		if m.step == len(flipSteps)/2 {
			m.back = m.hover
		}
		if m.atRest() {
			return m, nil
		}
		return m, tick(flipFrame)
	}
	return m, nil
}

func (m bannerModel) View() string {
	if m.intro > 0 {
		return renderIntro(introTotal - m.intro)
	}
	art := bannerFront
	if m.back {
		art = bannerBack
	}
	w := flipSteps[min(m.step, len(flipSteps)-1)]
	var side []string
	if w == bannerWidth {
		if m.back {
			side = sideBack()
		} else {
			side = sideFront()
		}
	}
	view := "\n" + renderBanner(squash(art, w), side)
	if !m.done {
		view += "\n" + bannerDim.Render("  Press Enter to continue") + "\n"
	}
	return view
}

// bannerMinCols fits the glyph plus the longest side line; narrower terminals wrap and break in-place redraws.
const bannerMinCols = bannerLeft + bannerWidth + 3 + 42

func showBanner(w io.Writer) error {
	switch {
	case output.ASCII:
		_, _ = fmt.Fprintln(w, "\n"+output.Cyan(output.LogoASCII)+"\n\nTeamCity CLI  v"+version.String()+"\nOnboarding · from zero to green build hero")
		return nil
	case output.NoColor || !output.IsTerminal():
		_, _ = fmt.Fprintln(w, "\n"+output.Logo+"\n\nTeamCity CLI  v"+version.String()+"\nOnboarding · from zero to green build hero")
		return nil
	}
	// The renderer only erases stale line tails when it knows the width, so animate only on a sized, wide-enough tty.
	if cols, _, err := term.GetSize(int(os.Stdout.Fd())); err != nil || cols < bannerMinCols {
		_, _ = fmt.Fprint(w, "\n"+renderBanner(squash(bannerFront, bannerWidth), sideFront()))
		return nil
	}
	output.StopSpinner()
	m := bannerModel{intro: introTotal, step: len(flipSteps) - 1}
	// Default output is os.Stdout itself: a wrapped writer hides the tty, and with it the window size.
	final, err := tea.NewProgram(m, tea.WithMouseAllMotion()).Run()
	if err != nil {
		return err
	}
	if final.(bannerModel).aborted {
		return huh.ErrUserAborted
	}
	return nil
}
