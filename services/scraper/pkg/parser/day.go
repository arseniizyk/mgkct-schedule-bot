package parser

import (
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
