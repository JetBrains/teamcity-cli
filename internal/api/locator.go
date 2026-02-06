package api

import (
	"fmt"
	"net/url"
	"strings"
)

type Locator struct {
	parts []string
}

func NewLocator() *Locator {
	return &Locator{}
}

func (l *Locator) Add(key, value string) *Locator {
	if value != "" {
		l.parts = append(l.parts, fmt.Sprintf("%s:%s", key, escapeLocatorValue(value)))
	}
	return l
}

// escapeLocatorValue wraps values containing special characters in parentheses.
// TeamCity locator syntax uses : and , as delimiters, and $ as escape char
// inside parenthesized values ($) for literal ), $$ for literal $).
func escapeLocatorValue(value string) string {
	if !strings.ContainsAny(value, ":,()$") {
		return value
	}
	// Escape $ first (so we don't double-escape), then )
	escaped := strings.ReplaceAll(value, "$", "$$")
	escaped = strings.ReplaceAll(escaped, ")", "$)")
	return "(" + escaped + ")"
}

func (l *Locator) AddUpper(key, value string) *Locator {
	if value != "" {
		l.parts = append(l.parts, fmt.Sprintf("%s:%s", key, strings.ToUpper(value)))
	}
	return l
}

func (l *Locator) AddInt(key string, value int) *Locator {
	if value > 0 {
		l.parts = append(l.parts, fmt.Sprintf("%s:%d", key, value))
	}
	return l
}

func (l *Locator) AddIntDefault(key string, value, defaultVal int) *Locator {
	if value > 0 {
		l.parts = append(l.parts, fmt.Sprintf("%s:%d", key, value))
	} else {
		l.parts = append(l.parts, fmt.Sprintf("%s:%d", key, defaultVal))
	}
	return l
}

func (l *Locator) String() string {
	return strings.Join(l.parts, ",")
}

func (l *Locator) Encode() string {
	return url.QueryEscape(l.String())
}

func (l *Locator) IsEmpty() bool {
	return len(l.parts) == 0
}
