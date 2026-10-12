package parser

import (
	"html"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

var (
	brRe  = regexp.MustCompile(`(?i)<br\s*/?>`)
	tagRe = regexp.MustCompile(`<[^>]*>`)
)

func parseRows(trs *goquery.Selection) []*entities.Day {
	res := make([]*entities.Day, 0, 6)

	ths := trs.Eq(0).Find("th")
	for col := 1; col < ths.Length(); col++ {
		day := strings.ReplaceAll(ths.Eq(col).Text(), ",", " |")
		res = append(res, &entities.Day{Name: cleanText(day)})
	}

	for row := 2; row < trs.Length(); row++ {
		tr := trs.Eq(row)

		num, ok := parsePairNumber(tr)
		if !ok {
			if len(res) == 0 {
				continue
			}
			num = len(res[0].Subjects) + 1
		}

		ensurePairIndex(res, num)
		parseColumns(tr.Find("td"), res)
	}

	return res
}

func parsePairNumber(tr *goquery.Selection) (int, bool) {
	th := tr.Find("th").First()
	if th.Length() == 0 {
		return 0, false
	}

	num, err := strconv.Atoi(cleanText(th.Text()))
	if err != nil || num < 1 {
		return 0, false
	}

	return num, true
}

func ensurePairIndex(days []*entities.Day, num int) {
	for _, d := range days {
		for len(d.Subjects) < num-1 {
			d.Subjects = append(d.Subjects, &entities.Subject{IsEmpty: true})
		}
	}
}

func parseColumns(tds *goquery.Selection, days []*entities.Day) {
	for col := 0; col < tds.Length(); col += 2 {
		daysIdx := col / 2
		if daysIdx >= len(days) {
			break
		}

		nameParts := splitByBr(tds.Eq(col))
		classParts := splitByBr(tds.Eq(col + 1))

		if len(nameParts) == 0 {
			days[daysIdx].Subjects = append(days[daysIdx].Subjects, &entities.Subject{IsEmpty: true})
			continue
		}

		pairs := parsePairs(nameParts, classParts)
		days[daysIdx].Subjects = append(days[daysIdx].Subjects, &entities.Subject{
			Pairs:   pairs,
			IsEmpty: pairsAreEmpty(pairs),
		})
	}
}

func pairsAreEmpty(pairs []*entities.Pair) bool {
	for _, p := range pairs {
		name := strings.TrimSpace(p.Name)
		hasContent := (name != "" && name != "-" && name != "—" && name != "–") || p.Group != ""
		if hasContent {
			return false
		}
	}
	return true
}

func parsePairs(nameParts, classParts []string) []*entities.Pair {
	var pairs []*entities.Pair

	for i := 0; i < len(nameParts); {
		var subjectType, groupNum, class string
		name := nameParts[i]
		i++

		if len(name) > 3 && name[1] == '.' {
			name = name[:2] + " " + name[2:]
		}

		if i < len(nameParts) && strings.HasPrefix(nameParts[i], "(") {
			subjectType = nameParts[i]
			subjectType, _ = strings.CutPrefix(subjectType, "(")
			subjectType, _ = strings.CutSuffix(subjectType, ")")
			i++
		}

		if i < len(nameParts) {
			groupNum = nameParts[i]
			i++
		}

		if len(pairs) < len(classParts) {
			class = strings.ReplaceAll(classParts[len(pairs)], "(к)", "")
		}

		pairs = append(pairs, &entities.Pair{
			Name:  cleanText(name),
			Type:  cleanText(subjectType),
			Group: cleanText(groupNum),
			Class: cleanText(class),
		})
	}

	return pairs
}

func splitByBr(td *goquery.Selection) []string {
	htmlContent, err := td.Html()
	if err != nil {
		slog.Warn("splitByBr: can't get html content", "err", err)
		return nil
	}

	parts := brRe.Split(htmlContent, -1)
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		cleaned := cleanText(p)
		if cleaned != "" {
			res = append(res, cleaned)
		}
	}
	return res
}

func cleanText(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}
