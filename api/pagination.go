package api

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

func paginatedFieldsParam(itemKey string, fields []string) string {
	return fmt.Sprintf("count,href,nextHref,%s(%s)", itemKey, ToAPIFields(fields))
}

func rewriteContinuationPath(path string, limit int, fieldsParam string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse continuation path: %w", err)
	}

	query := u.Query()
	query.Set("fields", fieldsParam)

	if limit > 0 {
		locator := query.Get("locator")
		if locator != "" {
			query.Set("locator", setLocatorInt(locator, "count", limit))
		}
	}

	u.RawQuery = query.Encode()
	return u.RequestURI(), nil
}

func setLocatorInt(locator, key string, value int) string {
	prefix := key + ":"
	replacement := fmt.Sprintf("%s%d", prefix, value)
	parts := splitLocator(locator)
	index := slices.IndexFunc(parts, func(part string) bool {
		rest, ok := strings.CutPrefix(part, prefix)
		if !ok {
			return false
		}
		_, err := strconv.Atoi(rest)
		return err == nil
	})
	if index >= 0 {
		parts[index] = replacement
	} else {
		parts = append(parts, replacement)
	}
	return strings.Join(parts, ",")
}

func splitLocator(locator string) []string {
	if locator == "" {
		return nil
	}

	parts := make([]string, 0, 8)
	var current strings.Builder
	depth := 0
	escaped := false

	for _, r := range locator {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		switch r {
		case '$':
			current.WriteRune(r)
			escaped = true
		case '(':
			depth++
			current.WriteRune(r)
		case ')':
			depth = max(depth-1, 0)
			current.WriteRune(r)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
				continue
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
