package parser

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gocolly/colly"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

func TestParseMultipleWeeksHTML(t *testing.T) {
	t.Parallel()

	htmlFixture := `
<!DOCTYPE html>
<html>
<body>
  <h2>Группа - 101</h2>
  <h3>05.10.2026 - 11.10.2026</h3>
  <table>
    <tbody>
      <tr><th>№</th><th>Понедельник, 05.10.2026</th></tr>
      <tr><th class="sub">Дисциплина</th><th class="sub">Ауд.</th></tr>
      <tr><th>1</th><td>Математика<br />(Лек)<br />Препод 1</td><td>101</td></tr>
    </tbody>
  </table>

  <h2>Группа - 101</h2>
  <h3>12.10.2026 - 18.10.2026</h3>
  <table>
    <tbody>
      <tr><th>№</th><th>Понедельник, 12.10.2026</th></tr>
      <tr><th class="sub">Дисциплина</th><th class="sub">Ауд.</th></tr>
      <tr><th>1</th><td>Информатика<br />(Лаб)<br />Препод 2</td><td>102</td></tr>
    </tbody>
  </table>
</body>
</html>
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(htmlFixture))
	}))
	defer server.Close()

	parser := New(nil)

	// Подменяем клиент на обращение к тестовому серверу
	col := parser.c.Clone()

	schedules := make(map[time.Time]*entities.Schedule)
	week1 := time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC)
	week2 := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)

	col.OnHTML("h2", func(e *colly.HTMLElement) {
		groupNum, err := parseGroup(e.Text)
		if err != nil {
			return
		}
		h3Sel := e.DOM.NextAllFiltered("h3").First()
		week, err := parseWeek(h3Sel.Text())
		if err != nil {
			return
		}
		tableSel := e.DOM.NextAllFiltered("table").First()
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

	if err := col.Visit(server.URL); err != nil {
		t.Fatalf("col.Visit failed: %v", err)
	}
	col.Wait()

	if len(schedules) != 2 {
		t.Fatalf("expected 2 distinct weeks parsed, got %d", len(schedules))
	}

	sch1, ok1 := schedules[week1]
	if !ok1 || sch1.Groups[101] == nil {
		t.Fatalf("week 1 (05.10) missing or group 101 nil")
	}
	if sch1.Groups[101].Days[0].Subjects[0].Pairs[0].Name != "Математика" {
		t.Errorf("expected week 1 pair to be Математика, got %s", sch1.Groups[101].Days[0].Subjects[0].Pairs[0].Name)
	}

	sch2, ok2 := schedules[week2]
	if !ok2 || sch2.Groups[101] == nil {
		t.Fatalf("week 2 (12.10) missing or group 101 nil")
	}
	if sch2.Groups[101].Days[0].Subjects[0].Pairs[0].Name != "Информатика" {
		t.Errorf("expected week 2 pair to be Информатика, got %s", sch2.Groups[101].Days[0].Subjects[0].Pairs[0].Name)
	}
}
