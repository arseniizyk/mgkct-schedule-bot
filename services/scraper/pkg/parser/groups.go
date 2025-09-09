package parser

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/gocolly/colly"
)

func parseGroup(text string) (int, error) {
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

func parseWeek(e *colly.HTMLElement) string {
	return cleanText(e.DOM.NextFiltered("h3").Text())
}

func parseTable(e *colly.HTMLElement) (*models.Group, error) {
	table := e.DOM.NextAllFiltered("table").First()

	week := parseWeek(e)
	days := parseRows(table.Find("tbody tr"))

	return &models.Group{
		Week: week,
		Days: days,
	}, nil
}
