package crawler

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/gocolly/colly"
)

func (c *Crawler) Crawl() (*models.Schedule, error) {
	schedule := models.Schedule{
		Groups: make(map[int]models.Group),
	}

	c.c.OnError(func(r *colly.Response, err error) {
		slog.Error("visit error", "url", r.Request.URL, "err", err)
	})

	c.c.OnHTML("h2", func(e *colly.HTMLElement) {
		groupNum, err := parseGroup(e.Text)
		if err != nil {
			slog.Error("can't get group from h2", "err", err)
			return
		}

		group, err := parseTable(e)
		if err != nil {
			slog.Warn("can't parse group", "err", err)
			return
		}

		schedule.Groups[groupNum] = *group
	})

	if err := c.c.Visit(url); err != nil {
		return nil, fmt.Errorf("visit failed: %w", err)
	}

	return &schedule, nil
}

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

		name := cleanText(tds.Eq(col).Text())

		var class string
		if col+1 < tds.Length() {
			class = tds.Eq(col + 1).Text()
			if isDash(class) {
				class = ""
			}
		}

		if isEmpty(name) {
			days[daysIdx].Subjects = append(days[daysIdx].Subjects, models.Subject{IsEmpty: true})
			continue
		}

		days[daysIdx].Subjects = append(days[daysIdx].Subjects, models.Subject{
			Name:    name,
			Class:   class,
			IsEmpty: false,
		})
	}
}
