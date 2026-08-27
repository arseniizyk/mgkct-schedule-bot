package parser

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

func newSelectionText(text string) *goquery.Selection {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<p>" + text + "</p>"))
	if err != nil {
		panic(err)
	}
	return doc.Find("p").First()
}

func TestParseGroup(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    int32
		wantErr error
	}{
		{name: "simple group", text: "Группа 99", want: 99},
		{name: "group with trailing text", text: "Группа 123 (П)", want: 123},
		{name: "only digits", text: "42", want: 42},
		{name: "bad group Кол", text: "Колледж 5", wantErr: ErrBadGroup},
		{name: "bad group кол lowercase", text: "кол хор", wantErr: ErrBadGroup},
		{name: "no digits", text: "Группа без цифр", wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGroup(tt.text)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseGroup(%q) error = %v, want %v", tt.text, err, tt.wantErr)
				}
				return
			}

			if tt.text == "Группа без цифр" {
				if err == nil {
					t.Fatalf("parseGroup(%q) expected error for no digits", tt.text)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseGroup(%q) unexpected error: %v", tt.text, err)
			}
			if got != tt.want {
				t.Errorf("parseGroup(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseWeek(t *testing.T) {
	t.Run("valid week header", func(t *testing.T) {
		sel := newSelectionText("24.08.2026 - 30.08.2026")

		got, err := parseWeek(sel)
		if err != nil {
			t.Fatalf("parseWeek() unexpected error: %v", err)
		}

		want := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("parseWeek() = %v, want %v", got, want)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		sel := newSelectionText("не дата - 30.08.2026")

		if _, err := parseWeek(sel); err == nil {
			t.Error("parseWeek() expected error for invalid date")
		}
	})
}

// buildTable собирает таблицу расписания в формате сайта:
// строка заголовков (№ + дни), затем строки данных "<th>№</th>" + ячейки пар.
func buildTable(numbers []int, subject string) string {
	var sb strings.Builder

	sb.WriteString("<table><tbody>")
	sb.WriteString("<tr><th>№</th><th>Понедельник, 31.08.2026</th></tr>")
	sb.WriteString("<tr><td class=\"sub\">Дисциплина</td><td class=\"sub\">Ауд.</td></tr>")

	for _, n := range numbers {
		fmt.Fprintf(&sb,
			"<tr><th>%d</th><td>%s<br />(Лек)<br />Препод Т. Т.</td><td class=\"sub\">3-113</td></tr>",
			n, subject)
	}

	sb.WriteString("</tbody></table>")
	return sb.String()
}

func parseTable(t *testing.T, html string) []*pb.Day {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	return parseRows(doc.Find("tbody tr"))
}

func TestParseRowsStandardLayout(t *testing.T) {
	days := parseTable(t, buildTable([]int{1, 2, 3}, "Математика"))

	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}

	subjects := days[0].GetSubjects()
	if len(subjects) != 3 {
		t.Fatalf("got %d subjects, want 3", len(subjects))
	}

	for i, s := range subjects {
		if s.GetIsEmpty() {
			t.Errorf("subject[%d] should not be empty", i)
			continue
		}
		if got := subjects[i].GetPairs()[0].GetName(); got != "Математика" {
			t.Errorf("subject[%d] name = %q", i, got)
		}
	}
}

func TestParseRowsSecondShiftLayout(t *testing.T) {
	days := parseTable(t, buildTable([]int{4, 5, 6, 7}, "Физика"))

	subjects := days[0].GetSubjects()
	if len(subjects) != 7 {
		t.Fatalf("got %d subjects, want 7 (с паддингом пустых слотов)", len(subjects))
	}

	for i := range 3 {
		if !subjects[i].GetIsEmpty() {
			t.Errorf("subject[%d] должен быть пустым слотом второй смены", i)
		}
	}

	for i := 3; i < 7; i++ {
		if subjects[i].GetIsEmpty() {
			t.Errorf("subject[%d] не должен быть пустым", i)
			continue
		}
		if got := subjects[i].GetPairs()[0].GetName(); got != "Физика" {
			t.Errorf("subject[%d] name = %q, want Физика (номер пары = индекс+1)", i, got)
		}
	}
}

func TestParseRowsNoStateBetweenCalls(t *testing.T) {
	// регрессия глобального кэша имён дней: разные таблицы не должны влиять друг на друга
	first := parseTable(t, buildTable([]int{1}, "Первый"))
	second := parseTable(t, buildTable([]int{4}, "Второй"))

	if len(first[0].GetSubjects()) != 1 || len(second[0].GetSubjects()) != 4 {
		t.Fatalf("subjects lengths = %d и %d, want 1 и 4",
			len(first[0].GetSubjects()), len(second[0].GetSubjects()))
	}

	if got := second[0].GetSubjects()[3].GetPairs()[0].GetName(); got != "Второй" {
		t.Errorf("subject[3] name = %q, want Второй", got)
	}
}
