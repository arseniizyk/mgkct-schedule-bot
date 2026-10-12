package parser

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

const (
	studentURL     = `https://mgkct.minskedu.gov.by/personnel/for-students/weekly-timetable`
	requestTimeout = 15 * time.Second
)

var (
	ErrBadGroup = errors.New("bad group")
	digitsRe    = regexp.MustCompile(`\d+`)
)

type Parser struct {
	log *slog.Logger
	c   *colly.Collector
}

func New(log *slog.Logger) *Parser {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"),
	)

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
		ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		},
	})

	c.SetRequestTimeout(requestTimeout)

	return &Parser{
		c:   c,
		log: log,
	}
}

// Parse парсит страницу расписания студентов и возвращает карту расписаний по неделям.
// Если на странице выложено 2 недели (текущая и следующая), возвращаются обе недели.
func (p *Parser) Parse(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	log := p.log.With("operation", "parser.Parser.Parse")

	col := p.c.Clone()

	schedules := make(map[time.Time]*entities.Schedule)

	col.OnError(func(r *colly.Response, err error) {
		log.ErrorContext(ctx, "visit error", "url", r.Request.URL, "error", err)
	})

	col.OnHTML("h2", func(e *colly.HTMLElement) {
		groupNum, err := parseGroup(e.Text)
		if err != nil {
			if errors.Is(err, ErrBadGroup) {
				return
			}
			log.WarnContext(ctx, "can't get group from h2", "text", e.Text, "error", err)
			return
		}

		h3Sel := e.DOM.NextAllFiltered("h3").First()
		if h3Sel.Length() == 0 {
			log.WarnContext(ctx, "no h3 found after h2 for group", "group", groupNum)
			return
		}

		week, err := parseWeek(h3Sel.Text())
		if err != nil {
			log.WarnContext(ctx, "can't parse week", "text", h3Sel.Text(), "error", err)
			return
		}

		tableSel := e.DOM.NextAllFiltered("table").First()
		if tableSel.Length() == 0 {
			log.WarnContext(ctx, "no table found after h2 for group", "group", groupNum)
			return
		}

		days := parseRows(tableSel.Find("tbody tr"))

		group := &entities.Group{
			ID:   groupNum,
			Week: week,
			Days: days,
		}

		if schedules[week] == nil {
			schedules[week] = &entities.Schedule{
				Groups: make(map[int32]*entities.Group),
			}
		}

		schedules[week].Groups[groupNum] = group
	})

	if err := col.Visit(studentURL); err != nil {
		return nil, fmt.Errorf("visit failed: %w", err)
	}

	col.Wait()

	if len(schedules) == 0 {
		return nil, fmt.Errorf("no schedules parsed from %s", studentURL)
	}

	return schedules, nil
}

func parseGroup(text string) (int32, error) {
	if strings.Contains(text, "Кол") || strings.Contains(text, "кол") {
		return 0, ErrBadGroup
	}

	matched := digitsRe.FindString(text)
	if matched == "" {
		return 0, fmt.Errorf("no digits in %q", text)
	}

	group, err := strconv.Atoi(matched)
	if err != nil {
		return 0, fmt.Errorf("can't parse group(%s) to int: %w", text, err)
	}

	return int32(group), nil
}

func parseWeek(text string) (time.Time, error) {
	layout := "02.01.2006"

	parts := strings.Split(text, " - ")
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid week string: %q", text)
	}

	startStr := strings.TrimSpace(parts[0])
	start, err := time.Parse(layout, startStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse week: %w", err)
	}

	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC), nil
}

// parseWeekSelection для обратной совместимости с тестами
func parseWeekSelection(e *goquery.Selection) (time.Time, error) {
	return parseWeek(e.Text())
}
