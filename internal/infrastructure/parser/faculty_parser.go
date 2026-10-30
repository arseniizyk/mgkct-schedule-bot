package parser

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

var facultyPages = []struct {
	URL        string
	Department string
}{
	{
		URL:        "https://mgkct.minskedu.gov.by/about/collective/teachers",
		Department: "teachers",
	},
	{
		URL:        "https://mgkct.minskedu.gov.by/about/collective/administration",
		Department: "administration",
	},
	{
		URL:        "https://mgkct.minskedu.gov.by/about/collective/employees",
		Department: "employees",
	},
}

type FacultyParser struct {
	log *slog.Logger
	c   *colly.Collector
}

func NewFacultyParser(log *slog.Logger) *FacultyParser {
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

	return &FacultyParser{
		log: log,
		c:   c,
	}
}

// Parse обходит страницы педсостава, администрации и сотрудников и возвращает список сотрудников
func (fp *FacultyParser) Parse(ctx context.Context) ([]*entities.FacultyMember, error) {
	log := fp.log.With("operation", "parser.FacultyParser.Parse")
	var allMembers []*entities.FacultyMember

	for _, page := range facultyPages {
		col := fp.c.Clone()
		var pageMembers []*entities.FacultyMember

		col.OnError(func(r *colly.Response, err error) {
			log.WarnContext(ctx, "failed fetching faculty page", "url", r.Request.URL, "error", err)
		})

		col.OnHTML(".card-body", func(e *colly.HTMLElement) {
			titleSel := e.DOM.Find(".employee-card-title")
			if titleSel.Length() == 0 {
				return
			}

			fullName := strings.TrimSpace(titleSel.Text())
			if fullName == "" {
				return
			}

			member := parseEmployeeCard(e.DOM, fullName, page.Department)
			if member != nil {
				pageMembers = append(pageMembers, member)
			}
		})

		if err := col.Visit(page.URL); err != nil {
			log.WarnContext(ctx, "visit error for faculty page", "url", page.URL, "error", err)
			continue
		}

		allMembers = append(allMembers, pageMembers...)
	}

	return allMembers, nil
}

// ParseHTML позволяет спарсить HTML напрямую (для тестов или кастомных источников)
func (fp *FacultyParser) ParseHTML(doc *goquery.Document, department string) []*entities.FacultyMember {
	var members []*entities.FacultyMember
	doc.Find(".card-body").Each(func(_ int, s *goquery.Selection) {
		titleSel := s.Find(".employee-card-title")
		if titleSel.Length() == 0 {
			return
		}
		fullName := strings.TrimSpace(titleSel.Text())
		if fullName == "" {
			return
		}
		m := parseEmployeeCard(s, fullName, department)
		if m != nil {
			members = append(members, m)
		}
	})
	return members
}

func parseEmployeeCard(s *goquery.Selection, fullName, department string) *entities.FacultyMember {
	shortName := ToShortName(fullName)
	if shortName == "" {
		return nil
	}

	member := &entities.FacultyMember{
		FullName:   fullName,
		ShortName:  shortName,
		Department: department,
	}

	// Позиция / Должность (например <p ...><strong class="text-primary"><i class="fa-solid fa-briefcase ..."></i>Преподаватель</strong></p>)
	briefcaseSel := s.Find("i.fa-briefcase").Parent()
	if briefcaseSel.Length() > 0 {
		member.Position = strings.TrimSpace(briefcaseSel.Text())
	}

	// Обход параграфов для извлечения квалификации, образования, стажа
	s.Find("p").Each(func(_ int, p *goquery.Selection) {
		text := strings.TrimSpace(p.Text())
		switch {
		case strings.Contains(text, "Квалификация:"):
			member.Qualification = cleanFieldText(text, "Квалификация:")
		case strings.Contains(text, "Образование:"):
			member.Education = cleanFieldText(text, "Образование:")
		case strings.Contains(text, "Стаж:"):
			member.Experience = cleanFieldText(text, "Стаж:")
		}
	})

	return member
}

func cleanFieldText(text, label string) string {
	_, after, found := strings.Cut(text, label)
	if !found {
		return ""
	}
	return strings.TrimSpace(after)
}

// ToShortName преобразует полное ФИО "Иванов Иван Иванович" в "Иванов И. И."
func ToShortName(fullName string) string {
	parts := strings.Fields(strings.TrimSpace(fullName))
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	runes1 := []rune(parts[1])
	if len(parts) == 2 {
		if len(runes1) > 0 {
			return fmt.Sprintf("%s %c.", parts[0], runes1[0])
		}
		return parts[0]
	}

	runes2 := []rune(parts[2])
	if len(runes1) > 0 && len(runes2) > 0 {
		return fmt.Sprintf("%s %c. %c.", parts[0], runes1[0], runes2[0])
	}
	if len(runes1) > 0 {
		return fmt.Sprintf("%s %c.", parts[0], runes1[0])
	}
	return parts[0]
}
