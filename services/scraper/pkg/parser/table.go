package parser

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
)

func parseRows(trs *goquery.Selection) []models.Day {
	res := make([]models.Day, 6)
	for i, d := range days {
		res[i] = models.Day{Name: d}
	}

	for row := 2; row < trs.Length(); row++ {
		tds := trs.Eq(row).Find("td")
		parseColumns(tds, res)
	}

	return res
}

func parseColumns(tds *goquery.Selection, days []models.Day) {
	for col := 0; col < tds.Length(); col += 2 {
		daysIdx := col / 2

		nameParts := splitByBr(tds.Eq(col))
		classParts := splitByBr(tds.Eq(col + 1))

		if len(nameParts) == 0 {
			days[daysIdx].Subjects = append(days[daysIdx].Subjects, models.Subject{IsEmpty: true})
			continue
		}

		pairs := parsePairs(nameParts, classParts)
		days[daysIdx].Subjects = append(days[daysIdx].Subjects, models.Subject{
			Pairs:   pairs,
			IsEmpty: false,
		})
	}
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
