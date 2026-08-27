package parser

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/gocolly/colly"
)

const teacherURL = `https://mgkct.minskedu.gov.by/personnel/for-teachers/weekly-timetable`

var (
	ErrBadTeacher = errors.New("нет преподавателя")
	teacherRe     = regexp.MustCompile(`Преподаватель\s*[-–]\s*(.+)`)
)

type TeacherSchedule struct {
	Name string
	Week time.Time
	Days []*pb.Day
}

type TeacherParser struct {
	log *slog.Logger
	c   *colly.Collector
}

const defaultRequestTimeout = time.Second * 15

func NewTeacherParser(log *slog.Logger) *TeacherParser {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
	)

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
		ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		},
	})

	c.SetRequestTimeout(defaultRequestTimeout)

	return &TeacherParser{
		c:   c,
		log: log,
	}
}

func (tp *TeacherParser) Parse(ctx context.Context) ([]TeacherSchedule, *time.Time, error) {
	log := tp.log.With("operation", "infrastructure.parser.TeacherParser.Parse")

	col := tp.c.Clone()

	var schedules []TeacherSchedule
	var week time.Time

	col.OnError(func(r *colly.Response, err error) {
		log.ErrorContext(ctx, "visit error", "url", r.Request.URL, "error", err)
	})

	col.OnHTML("h2", func(e *colly.HTMLElement) {
		teacherName, err := parseTeacherName(e.Text)
		if err != nil {
			if errors.Is(err, ErrBadTeacher) {
				return
			}

			log.ErrorContext(ctx, "can't get teacher from h2", "error", err)
			return
		}

		table := e.DOM.NextAllFiltered("table").First()

		if week.IsZero() {
			week, err = parseWeek(e.DOM.Next())
			if err != nil {
				log.ErrorContext(ctx, "can't parse week", "error", err)
				week = time.Now()
			}
		}

		days := parseTeacherRows(table.Find("tbody tr"))

		schedules = append(schedules, TeacherSchedule{
			Name: teacherName,
			Week: week,
			Days: days,
		})
	})

	if err := col.Visit(teacherURL); err != nil {
		return nil, nil, fmt.Errorf("visit failed: %w", err)
	}

	col.Wait()

	return schedules, &week, nil
}

func parseTeacherName(text string) (string, error) {
	text = strings.TrimSpace(text)

	if strings.Contains(text, "Кол") || strings.Contains(text, "кол") {
		return "", ErrBadTeacher
	}

	matches := teacherRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		return "", fmt.Errorf("no teacher name in %q", text)
	}

	return strings.TrimSpace(matches[1]), nil
}

func parseTeacherRows(trs *goquery.Selection) []*pb.Day {
	res := make([]*pb.Day, 0, 6)

	ths := trs.Eq(0).Find("th")
	for col := 1; col < ths.Length(); col++ {
		day := strings.ReplaceAll(ths.Eq(col).Text(), ",", " |")
		res = append(res, &pb.Day{Name: day})
	}

	for row := 2; row < trs.Length(); row++ {
		tr := trs.Eq(row)

		num, ok := parsePairNumber(tr)
		if !ok {
			if len(res) == 0 {
				continue
			}
			num = len(res[0].GetSubjects()) + 1
		}

		ensurePairIndex(res, num)

		parseTeacherColumns(tr.Find("td"), res)
	}

	return res
}

func parseTeacherColumns(tds *goquery.Selection, days []*pb.Day) {
	for col := 0; col < tds.Length(); col += 2 {
		daysIdx := col / 2

		nameParts := splitByBr(tds.Eq(col))
		classParts := splitByBr(tds.Eq(col + 1))

		if len(nameParts) == 0 {
			days[daysIdx].Subjects = append(days[daysIdx].Subjects, &pb.Subject{IsEmpty: true})
			continue
		}

		pairs := parseTeacherPairs(nameParts, classParts)
		days[daysIdx].Subjects = append(days[daysIdx].Subjects, &pb.Subject{
			Pairs:   pairs,
			IsEmpty: pairsAreEmpty(pairs),
		})
	}
}

func parseTeacherPairs(nameParts, classParts []string) []*pb.Pair {
	var pairs []*pb.Pair

	for i := 0; i < len(nameParts); {
		var subjectType string
		rawName := nameParts[i]
		i++

		if i < len(nameParts) && strings.HasPrefix(nameParts[i], "(") {
			subjectType = nameParts[i]
			subjectType, _ = strings.CutPrefix(subjectType, "(")
			subjectType, _ = strings.CutSuffix(subjectType, ")")
			i++
		}

		group, subjectName := splitTeacherCellSubject(rawName)

		var class string
		if len(pairs) < len(classParts) {
			class = classParts[len(pairs)]
			class = strings.ReplaceAll(class, "(к)", "")
		}

		pairs = append(pairs, &pb.Pair{
			Name:  cleanText(subjectName),
			Type:  cleanText(subjectType),
			Group: cleanText(group),
			Class: cleanText(class),
		})
	}

	return pairs
}

func splitTeacherCellSubject(raw string) (group, subject string) {
	raw = cleanText(raw)

	idx := strings.Index(raw, " - ")
	if idx < 0 {
		idx = strings.Index(raw, " – ")
	}
	if idx < 0 {
		return "", raw
	}

	group = strings.TrimSpace(raw[:idx])
	subject = strings.TrimSpace(raw[idx+3:])

	return group, subject
}
