package supports

import "strings"

func JoinURL(base string, segments ...string) string {
	if base == "" {
		return ""
	}
	parts := append([]string{strings.TrimRight(base, "/")}, segments...)
	return strings.Join(parts, "/")
}
