package cmdutil

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// RequireNonEmpty is a huh validator that rejects empty or whitespace-only input.
func RequireNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value is required")
	}
	return nil
}

// Prompt runs a single huh field with the CLI theme; does not echo — use PromptString / Select / Confirm for that.
func Prompt(field huh.Field) error {
	if in, ok := field.(*huh.Input); ok {
		in.Prompt("")
	}
	return huh.NewForm(huh.NewGroup(field)).
		WithTheme(promptTheme()).
		WithShowHelp(true).
		Run()
}

// PromptString asks for free-form text and echoes the answer back so it survives in scrollback.
func PromptString(p *output.Printer, title, description string, value *string) error {
	input := huh.NewInput().
		Title(title).
		Validate(RequireNonEmpty).
		Value(value)
	if description != "" {
		input.Description(description)
	}
	if err := Prompt(input); err != nil {
		return err
	}
	echo(p, title, *value)
	return nil
}

// PromptOptionalString asks for free-form text without rejecting empty input, useful when blank means "skip".
func PromptOptionalString(p *output.Printer, title, description string, value *string) error {
	input := huh.NewInput().
		Title(title).
		Value(value)
	if description != "" {
		input.Description(description)
	}
	if err := Prompt(input); err != nil {
		return err
	}
	echo(p, title, *value)
	return nil
}

// PromptSecret asks for a hidden value and never echoes it back.
func PromptSecret(title string, value *string) error {
	return Prompt(huh.NewInput().
		Title(title).
		EchoMode(huh.EchoModePassword).
		Validate(RequireNonEmpty).
		Value(value))
}

// Select presents a typed picker (filtering auto-enabled for 5+ options) and echoes the picked label back.
func Select[T comparable](p *output.Printer, title string, options []huh.Option[T], value *T) error {
	s := huh.NewSelect[T]().
		Title(title).
		Options(options...).
		Value(value)
	if len(options) >= 5 {
		s.Filtering(true)
	}
	if err := Prompt(s); err != nil {
		return err
	}
	for _, o := range options {
		if o.Value == *value {
			echo(p, title, o.Key)
			break
		}
	}
	return nil
}

// Confirm asks a yes/no question inline with left-aligned compact buttons.
func Confirm(title string, value *bool) error {
	return Prompt(huh.NewConfirm().
		Title(title).
		Affirmative("yes").
		Negative("no").
		Inline(true).
		WithButtonAlignment(lipgloss.Left).
		Value(value))
}

func echo(p *output.Printer, label, value string) {
	if value == "" || p == nil {
		return
	}
	_, _ = fmt.Fprintf(p.Out, "%s: %s\n", label, output.Cyan(value))
}

var (
	promptThemeOnce sync.Once
	promptThemeVal  *huh.Theme
)

// promptTheme renders huh prompts in the CLI's 16-color palette with no borders or magenta accents.
func promptTheme() *huh.Theme {
	promptThemeOnce.Do(func() {
		t := huh.ThemeBase()

		var (
			cyan   = lipgloss.Color("6")
			green  = lipgloss.Color("2")
			yellow = lipgloss.Color("3")
			red    = lipgloss.Color("1")
			faint  = lipgloss.Color("8")
			plain  = lipgloss.NewStyle()
		)

		t.Focused.Base = plain
		t.Focused.Card = plain
		t.Focused.Title = plain.Bold(true)
		t.Focused.NoteTitle = plain.Bold(true)
		t.Focused.Description = plain.Foreground(faint)
		t.Focused.ErrorIndicator = plain.Foreground(red).SetString(" ✗")
		t.Focused.ErrorMessage = plain.Foreground(red)

		t.Focused.SelectSelector = plain.Foreground(yellow).SetString("→ ")
		t.Focused.NextIndicator = plain.Foreground(yellow).MarginLeft(1).SetString("→")
		t.Focused.PrevIndicator = plain.Foreground(yellow).MarginRight(1).SetString("←")
		t.Focused.Option = plain
		t.Focused.SelectedOption = plain

		t.Focused.MultiSelectSelector = plain.Foreground(yellow).SetString("→ ")
		t.Focused.SelectedPrefix = plain.Foreground(green).SetString("✓ ")
		t.Focused.UnselectedPrefix = plain.Foreground(faint).SetString("• ")
		t.Focused.UnselectedOption = plain

		t.Focused.FocusedButton = plain.Bold(true).Foreground(cyan).MarginLeft(3)
		t.Focused.BlurredButton = plain.Foreground(faint).MarginLeft(3)

		t.Focused.TextInput.Cursor = plain.Foreground(cyan)
		t.Focused.TextInput.Placeholder = plain.Foreground(faint)
		t.Focused.TextInput.Prompt = plain.Foreground(yellow)

		t.Blurred = t.Focused
		t.Blurred.Base = plain
		t.Blurred.Card = plain
		t.Blurred.NextIndicator = plain
		t.Blurred.PrevIndicator = plain

		t.Group.Title = t.Focused.Title
		t.Group.Description = t.Focused.Description

		promptThemeVal = t
	})
	return promptThemeVal
}
