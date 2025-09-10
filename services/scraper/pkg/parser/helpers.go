package parser

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
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

func parsePairs(nameParts, classParts []string) []models.Pair {
	var pairs []models.Pair

	for i := 0; i < len(nameParts); {
		var subjectType, teacher, class string
		name := nameParts[i]
		i++

		if i < len(nameParts) && strings.HasPrefix(nameParts[i], "(") {
			subjectType = nameParts[i]
			subjectType, _ = strings.CutPrefix(subjectType, "(")
			subjectType, _ = strings.CutSuffix(subjectType, ")")
			i++
		}

		if i < len(nameParts) {
			teacher = nameParts[i]
			i++
		}

		class = classParts[len(pairs)]

		pairs = append(pairs, models.Pair{
			Name:    cleanText(name),
			Type:    cleanText(subjectType),
			Teacher: cleanText(teacher),
			Class:   cleanText(class),
		})
	}

	return pairs
}
