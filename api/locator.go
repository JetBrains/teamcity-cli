package api

import (
	"encoding/base64"
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

// escapeLocatorValue wraps values containing the : and , delimiters in parentheses and transports values containing ( ) $ as ($base64:<base64url>) — live servers reject the $)-style in-value escapes and name their decoder Base64url, so that is the only encoding that round-trips.
func escapeLocatorValue(value string) string {
	if !strings.ContainsAny(value, ":,()$") {
		return value
	}
	if !strings.ContainsAny(value, "()$") {
		return "(" + value + ")"
	}
	return "($base64:" + base64.RawURLEncoding.EncodeToString([]byte(value)) + ")"
}

// nameValueLocator builds the name:(value:($base64:...)) condition form for dimensions whose value is itself a locator (branch, pool, cloud profile/image) — there the server re-parses even a base64-decoded bare value as locator syntax, so the name must sit in an explicit value condition.
func nameValueLocator(value string) string {
	return "name:(value:($base64:" + base64.RawURLEncoding.EncodeToString([]byte(value)) + "))"
}

// AddRaw adds a key:value pair without escaping the value.
// Use for values that are already valid locator syntax (e.g. sub-locators).
func (l *Locator) AddRaw(key, value string) *Locator {
	if value != "" {
		l.parts = append(l.parts, fmt.Sprintf("%s:%s", key, value))
	}
	return l
}

// AddLocator adds a nested locator as key:(...)
func (l *Locator) AddLocator(key string, child *Locator) *Locator {
	if child != nil && !child.IsEmpty() {
		l.parts = append(l.parts, fmt.Sprintf("%s:%s", key, child.wrap()))
	}
	return l
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

func (l *Locator) wrap() string {
	return "(" + l.String() + ")"
}
