package parser

import (
	"fmt"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/gocolly/colly"
)

func (c *Parser) Parse() (*models.Schedule, error) {
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
