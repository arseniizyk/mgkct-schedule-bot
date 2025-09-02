package crawler

import "strings"

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", "")
	return strings.TrimSpace(s)
}

func isEmpty(s string) bool {
	return cleanText(s) == ""
}

func isDash(s string) bool {
	s = strings.TrimSpace(s)
	return s == "-" || s == "—" || s == "--"
}
