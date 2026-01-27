package parser

import (
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
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/gocolly/colly"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const url = `https://mgkct.minskedu.gov.by/personnel/for-students/weekly-timetable`

var (
	ErrBadGroup = errors.New("кол группа")
)

type Parser struct {
	log *slog.Logger
	c   *colly.Collector
}

func New(log *slog.Logger) *Parser {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
	)

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
		ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS12,
		},
	})

	return &Parser{
		c:   c,
		log: log,
	}
}

func (c *Parser) Parse() (*pb.Schedule, *time.Time, error) {
	log := c.log.With("operation", "infrastructure.parser.Parser.Parse")

	schedule := pb.Schedule{
		Groups: make(map[int32]*pb.Group),
	}

	c.c.OnError(func(r *colly.Response, err error) {
		log.Error("visit error", "url", r.Request.URL, "error", err)
	})

	var week time.Time

	c.c.OnHTML("h2", func(e *colly.HTMLElement) {
		groupNum, err := parseGroup(e.Text)
		if err != nil {
			if errors.Is(err, ErrBadGroup) {
				return
			}

			log.Error("can't get group from h2", "error", err)
			return
		}

		table := e.DOM.NextAllFiltered("table").First()

		week, err = parseWeek(e.DOM.Next()) // <h3>
		if err != nil {
			log.Error("can't parse week", "error", err)
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
		return 0, fmt.Errorf("can't parse group(%s) to int: %w", text, err)
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
