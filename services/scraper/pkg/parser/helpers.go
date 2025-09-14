package parser

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", "")
	return strings.TrimSpace(s)
}

func parseGroup(text string) (int, error) {
	if strings.Contains(text, "Кол") || strings.Contains(text, "кол") {
		return 0, ErrBadGroup
	}

	r := regexp.MustCompile(`\d+`)
	matched := r.FindString(text)
	if matched == "" {
		return 0, fmt.Errorf("no digits in %q", text)
	}

	group, err := strconv.Atoi(matched)
	if err != nil {
		slog.Error("can't parse group to int", "text", text, "err", err)
		return 0, err
	}

	return group, nil
}

func parseWeek(e *goquery.Selection) (time.Time, error) {
	layout := "02.01.2006"

	parts := strings.Split(e.Text(), " - ")
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid week string: %q", e.Text())
	}

	startStr := strings.TrimSpace(parts[0])
	start, err := time.Parse(layout, startStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse week: %w", err)
	}

	return start, nil
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
