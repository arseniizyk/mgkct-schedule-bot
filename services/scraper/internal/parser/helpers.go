package parser

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", "")
	return strings.TrimSpace(s)
}

func splitByBr(td *goquery.Selection) []string {
	html, err := td.Html()
	if err != nil {
		slog.Warn("splitByBr: can't get html content", "err", err)
		return nil
	}

	re := regexp.MustCompile(`(?i)<br\s*/?>`)
	parts := re.Split(html, -1)

	res := make([]string, 0, len(parts))
	for _, p := range parts {
		text := cleanText(p)
		if text != "" {
			res = append(res, text)
		}
	}
	return res
}
