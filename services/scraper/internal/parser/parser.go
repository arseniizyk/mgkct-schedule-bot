package parser

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/gocolly/colly"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func (c *Parser) Parse() (*pb.Schedule, *time.Time, error) {
	schedule := pb.Schedule{
		Groups: make(map[int32]*pb.Group),
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

		group := pb.Group{
			Id:   groupNum,
			Week: timestamppb.New(week),
			Days: parseRows(table.Find("tbody tr")),
		}

		schedule.Groups[groupNum] = &group
	})

	if err := c.c.Visit(url); err != nil {
		return nil, nil, fmt.Errorf("visit failed: %w", err)
	}

	return &schedule, &week, nil
}

func parseGroup(text string) (int32, error) {
	if strings.Contains(text, "Кол") || strings.Contains(text, "кол") {
		return 0, ErrBadGroup
	}

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

	return int32(group), nil
}

func parseWeek(e *goquery.Selection) (time.Time, error) {
	layout := "02.01.2006"

	parts := strings.Split(e.Text(), " - ")
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid week string: %q", e.Text())
	}

	startStr := strings.TrimSpace(parts[0])
	start, err := time.Parse(layout, startStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse week: %w", err)
	}

	return start, nil
}
