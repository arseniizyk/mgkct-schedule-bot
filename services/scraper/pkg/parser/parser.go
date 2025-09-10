package parser

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/gocolly/colly"
)

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

		week = parseWeek(e)

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
