package onboard

import (
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/output"
	tea "github.com/charmbracelet/bubbletea"
)

func TestBannerSquashAndFlip(t *testing.T) {
	for _, w := range []int{18, 9, 1, 0} {
		for _, l := range squash(bannerFront, w) {
			if n := len([]rune(l)); n != bannerWidth {
				t.Fatalf("w=%d width %d: %q", w, n, l)
			}
		}
	}
	if squash(bannerFront, 18)[0] != bannerFront[0]+" " {
		t.Fatalf("full width must be identity: %q", squash(bannerFront, 18)[0])
	}
	var m tea.Model = bannerModel{intro: introTotal, step: len(flipSteps) - 1}
	for range introTotal {
		m, _ = m.Update(bannerTick{})
	}
	if strings.ContainsAny(m.View(), string(output.LogoGlyphs[2:])) || !strings.Contains(m.View(), "TeamCity CLI") {
		t.Fatal("intro must end on the clean front face")
	}
	m, _ = m.Update(tea.MouseMsg{X: bannerLeft + 3, Action: tea.MouseActionMotion})
	for range len(flipSteps) {
		m, _ = m.Update(bannerTick{})
	}
	if !m.(bannerModel).back || !strings.Contains(m.View(), "YEARS OF RELIABLE CI") {
		t.Fatal("hover must flip to back face")
	}
	m, _ = m.Update(tea.MouseMsg{X: 60, Action: tea.MouseActionMotion})
	for range len(flipSteps) {
		m, _ = m.Update(bannerTick{})
	}
	if m.(bannerModel).back || !strings.Contains(m.View(), "TeamCity CLI") {
		t.Fatal("leave must flip to front face")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.(bannerModel).aborted || strings.Contains(m.View(), "Press Enter") {
		t.Fatal("ctrl+c must abort and hide the hint")
	}
}
