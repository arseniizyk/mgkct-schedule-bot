package crawler

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
)

const url = `https://mgkct.minskedu.gov.by/%D0%BF%D0%B5%D1%80%D1%81%D0%BE%D0%BD%D0%B0%D0%BB%D0%B8%D0%B8/%D1%83%D1%87%D0%B0%D1%89%D0%B8%D0%BC%D1%81%D1%8F/%D1%80%D0%B0%D1%81%D0%BF%D0%B8%D1%81%D0%B0%D0%BD%D0%B8%D0%B5-%D0%B7%D0%B0%D0%BD%D1%8F%D1%82%D0%B8%D0%B9-%D0%BD%D0%B0-%D0%BD%D0%B5%D0%B4%D0%B5%D0%BB%D1%8E`

var days = []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

type Crawler struct {
	c *colly.Collector
}

func New() *Crawler {
	return &Crawler{
		c: colly.NewCollector(),
	}
}

func (c *Crawler) Crawl() (*Schedule, error) {
	var schedule Schedule

	c.c.OnError(func(r *colly.Response, err error) {
		slog.Error("visit error", "url", r.Request.URL, "err", err)
	})

	c.c.OnHTML("h2", func(e *colly.HTMLElement) {
		group, err := parseTable(e)
		if err != nil {
			slog.Warn("can't parse group", "err", err)
			return
		}

		schedule.Groups = append(schedule.Groups, *group)
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

func parseTable(e *colly.HTMLElement) (*Group, error) {
	table := e.DOM.NextAllFiltered("table").First()

	groupNum, err := parseGroup(e.Text)
	if err != nil {
		slog.Error("can't get group from h2", "err", err)
		return nil, err
	}

	week := parseWeek(e)
	days := parseRows(table.Find("tbody tr"))

	return &Group{
		Number: groupNum,
		Week:   week,
		Days:   days,
	}, nil
}

func parseRows(trs *goquery.Selection) []Day {
	res := make([]Day, 6)
	for i, d := range days {
		res[i] = Day{Name: d}
	}

	for row := 2; row < trs.Length(); row++ {
		tds := trs.Eq(row).Find("td")
		parseColumns(tds, res)
	}

	return res
}

func parseColumns(tds *goquery.Selection, days []Day) {
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
			days[daysIdx].Subjects = append(days[daysIdx].Subjects, Subject{IsEmpty: true})
			continue
		}

		days[daysIdx].Subjects = append(days[daysIdx].Subjects, Subject{
			Name:    name,
			Class:   class,
			IsEmpty: false,
		})
	}
}
