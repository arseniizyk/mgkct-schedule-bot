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

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

const teacherURL = `https://mgkct.minskedu.gov.by/personnel/for-teachers/weekly-timetable`

var (
	ErrBadTeacher = errors.New("bad teacher")
	teacherRe     = regexp.MustCompile(`Преподаватель\s*[-–]\s*(.+)`)
)

type TeacherParser struct {
	log *slog.Logger
	c   *colly.Collector
}

func NewTeacherParser(log *slog.Logger) *TeacherParser {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"),
	)

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
		ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{
			MinVersion:       tls.VersionTLS12,
			MaxVersion:       tls.VersionTLS13,
			CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
	})

	c.SetRequestTimeout(requestTimeout)

	return &TeacherParser{
		c:   c,
		log: log,
	}
}

// Parse парсит страницу расписания преподавателей и возвращает всех преподавателей для всех недель на странице.
func (tp *TeacherParser) Parse(ctx context.Context) ([]*entities.Teacher, error) {
	log := tp.log.With("operation", "parser.TeacherParser.Parse")

	col := tp.c.Clone()

	var teachers []*entities.Teacher

	col.OnError(func(r *colly.Response, err error) {
		log.ErrorContext(ctx, "visit error", "url", r.Request.URL, "error", err)
	})

	col.OnHTML("h2", func(e *colly.HTMLElement) {
		teacherName, err := parseTeacherName(e.Text)
		if err != nil {
			if errors.Is(err, ErrBadTeacher) {
				return
			}
			log.WarnContext(ctx, "can't get teacher from h2", "text", e.Text, "error", err)
			return
		}

		h3Sel := e.DOM.NextAllFiltered("h3").First()
		if h3Sel.Length() == 0 {
			log.WarnContext(ctx, "no h3 found after h2 for teacher", "name", teacherName)
			return
		}

		week, err := parseWeek(h3Sel.Text())
		if err != nil {
			log.WarnContext(ctx, "can't parse week", "text", h3Sel.Text(), "error", err)
			return
		}

		tableSel := e.DOM.NextAllFiltered("table").First()
		if tableSel.Length() == 0 {
			log.WarnContext(ctx, "no table found after h2 for teacher", "name", teacherName)
			return
		}

		days := parseTeacherRows(tableSel.Find("tbody tr"))

		teachers = append(teachers, &entities.Teacher{
			Name: teacherName,
			Week: week,
			Days: days,
		})
	})

	if err := col.Visit(teacherURL); err != nil {
		return nil, fmt.Errorf("visit failed: %w", err)
	}

	col.Wait()

	if len(teachers) == 0 {
		return nil, fmt.Errorf("no teacher schedules parsed from %s", teacherURL)
	}

	return teachers, nil
}

func parseTeacherName(text string) (string, error) {
	text = cleanText(text)

	if strings.Contains(text, "Кол") || strings.Contains(text, "кол") {
		return "", ErrBadTeacher
	}

	matches := teacherRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		return "", fmt.Errorf("no teacher name in %q", text)
	}

	return strings.TrimSpace(matches[1]), nil
}

func parseTeacherRows(trs *goquery.Selection) []*entities.Day {
	res := make([]*entities.Day, 0, 6)

	ths := trs.Eq(0).Find("th")
	for col := 1; col < ths.Length(); col++ {
		day := strings.ReplaceAll(ths.Eq(col).Text(), ",", " |")
		res = append(res, &entities.Day{Name: cleanText(day)})
	}

	for row := 2; row < trs.Length(); row++ {
		tr := trs.Eq(row)

		num, ok := parsePairNumber(tr)
		if !ok {
			if len(res) == 0 {
				continue
			}
			num = len(res[0].Subjects) + 1
		}

		ensurePairIndex(res, num)
		parseTeacherColumns(tr.Find("td"), res)
	}

	return res
}

func parseTeacherColumns(tds *goquery.Selection, days []*entities.Day) {
	for col := 0; col < tds.Length(); col += 2 {
		daysIdx := col / 2
		if daysIdx >= len(days) {
			break
		}

		nameParts := splitByBr(tds.Eq(col))
		classParts := splitByBr(tds.Eq(col + 1))

		if len(nameParts) == 0 {
			days[daysIdx].Subjects = append(days[daysIdx].Subjects, &entities.Subject{IsEmpty: true})
			continue
		}

		pairs := parseTeacherPairs(nameParts, classParts)
		days[daysIdx].Subjects = append(days[daysIdx].Subjects, &entities.Subject{
			Pairs:   pairs,
			IsEmpty: pairsAreEmpty(pairs),
		})
	}
}

func parseTeacherPairs(nameParts, classParts []string) []*entities.Pair {
	var pairs []*entities.Pair

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

		pairs = append(pairs, &entities.Pair{
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
