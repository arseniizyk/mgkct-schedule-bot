package parser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

const teacherURL = `https://mgkct.minskedu.gov.by/personnel/for-teachers/weekly-timetable`

var (
	ErrBadTeacher      = errors.New("bad teacher")
	teacherRe          = regexp.MustCompile(`Преподаватель\s*[-–—]\s*(.+)`)
	teacherCellGroupRe = regexp.MustCompile(`^(\d+\*?)\s*[-–—]\s*(.+)$`)
)

type TeacherParser struct {
	log *slog.Logger
	c   *colly.Collector
}

func NewTeacherParser(log *slog.Logger) *TeacherParser {
	return &TeacherParser{
		c:   newCollector(),
		log: log,
	}
}

// Parse парсит страницу расписания преподавателей и возвращает всех преподавателей для всех недель на странице.
func (tp *TeacherParser) Parse(ctx context.Context) ([]*entities.Teacher, error) {
	log := tp.log.With("operation", "parser.TeacherParser.Parse")

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var teachers []*entities.Teacher
		col := tp.c.Clone()

		var visitErr error
		col.OnError(func(r *colly.Response, err error) {
			var urlStr string
			if r != nil && r.Request != nil && r.Request.URL != nil {
				urlStr = r.Request.URL.String()
			}
			visitErr = err
			log.WarnContext(ctx, "visit error", "url", urlStr, "attempt", attempt, "error", err)
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

			h3Sel := e.DOM.NextUntil("h2").Filter("h3").First()
			if h3Sel.Length() == 0 {
				log.WarnContext(ctx, "no h3 found after h2 for teacher", "name", teacherName)
				return
			}

			week, err := parseWeek(h3Sel.Text())
			if err != nil {
				log.WarnContext(ctx, "can't parse week", "text", h3Sel.Text(), "error", err)
				return
			}

			tableSel := e.DOM.NextUntil("h2").Filter("table").First()
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

		err := col.Visit(teacherURL)
		if err == nil {
			col.Wait()
		}

		switch {
		case err != nil:
			lastErr = err
		case visitErr != nil:
			lastErr = visitErr
		case len(teachers) == 0:
			lastErr = fmt.Errorf("no teacher schedules parsed from %s", teacherURL)
		default:
			return teachers, nil
		}

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
			}
		}
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

func parseTeacherName(text string) (string, error) {
	text = cleanText(text)

	matches := teacherRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		if strings.Contains(text, "Кол") || strings.Contains(text, "кол") {
			return "", ErrBadTeacher
		}
		return "", fmt.Errorf("no teacher name in %q", text)
	}

	name := strings.TrimSpace(matches[1])
	if name == "" {
		return "", ErrBadTeacher
	}

	return name, nil
}

func parseTeacherRows(trs *goquery.Selection) []*entities.Day {
	return parseRowsWith(trs, parseTeacherPairs)
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

	if matches := teacherCellGroupRe.FindStringSubmatch(raw); len(matches) == 3 {
		return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2])
	}

	if g, s, found := strings.Cut(raw, " - "); found {
		return strings.TrimSpace(g), strings.TrimSpace(s)
	}
	if g, s, found := strings.Cut(raw, " – "); found {
		return strings.TrimSpace(g), strings.TrimSpace(s)
	}
	if g, s, found := strings.Cut(raw, " — "); found {
		return strings.TrimSpace(g), strings.TrimSpace(s)
	}

	return "", raw
}
