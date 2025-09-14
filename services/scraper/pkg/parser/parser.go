package parser

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/gocolly/colly"
)

const url = `https://mgkct.minskedu.gov.by/%D0%BF%D0%B5%D1%80%D1%81%D0%BE%D0%BD%D0%B0%D0%BB%D0%B8%D0%B8/%D1%83%D1%87%D0%B0%D1%89%D0%B8%D0%BC%D1%81%D1%8F/%D1%80%D0%B0%D1%81%D0%BF%D0%B8%D1%81%D0%B0%D0%BD%D0%B8%D0%B5-%D0%B7%D0%B0%D0%BD%D1%8F%D1%82%D0%B8%D0%B9-%D0%BD%D0%B0-%D0%BD%D0%B5%D0%B4%D0%B5%D0%BB%D1%8E`

var (
	ErrBadGroup = errors.New("кол группа")
	days        = []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
)

type Parser struct {
	c *colly.Collector
}

func New() *Parser {
	return &Parser{
		c: colly.NewCollector(colly.AllowURLRevisit()),
	}
}

func (c *Parser) Parse() (*models.Schedule, *time.Time, error) {
	schedule := models.Schedule{
		Groups: make(map[int]models.Group),
	}

	c.c.OnError(func(r *colly.Response, err error) {
		slog.Error("visit error", "url", r.Request.URL, "err", err)
	})

	var week time.Time

	c.c.OnHTML("h2", func(e *colly.HTMLElement) {
		groupNum, err := parseGroup(e.Text)
		if err != nil {
			if errors.Is(err, ErrBadGroup) {
				return
			}

			slog.Error("can't get group from h2", "err", err)
			return
		}

		table := e.DOM.NextAllFiltered("table").First()

		week, err = parseWeek(e.DOM.Next()) // <h3>
		if err != nil {
			slog.Error("can't parse week", "err", err)
			week = time.Now()
		}

		group := models.Group{
			Week: week.Format("02-01-2006"),
			Days: parseRows(table.Find("tbody tr")),
		}

		schedule.Groups[groupNum] = group
	})

	if err := c.c.Visit(url); err != nil {
		return nil, nil, fmt.Errorf("visit failed: %w", err)
	}

	return &schedule, &week, nil
}
