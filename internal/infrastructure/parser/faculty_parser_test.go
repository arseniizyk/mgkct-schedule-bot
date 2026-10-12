package parser

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestToShortName(t *testing.T) {
	tests := []struct {
		full string
		want string
	}{
		{"Анципов Евгений Юрьевич", "Анципов Е. Ю."},
		{"Ковальчук Ольга Васильевна", "Ковальчук О. В."},
		{"Иванов Иван", "Иванов И."},
		{"Петров", "Петров"},
		{"", ""},
		{"   Сидоров   Сергей   Николаевич   ", "Сидоров С. Н."},
	}

	for _, tt := range tests {
		got := ToShortName(tt.full)
		if got != tt.want {
			t.Errorf("ToShortName(%q) = %q, want %q", tt.full, got, tt.want)
		}
	}
}

func TestFacultyParser_ParseHTML(t *testing.T) {
	const sampleHTML = `
<div class="card employee-card shadow-sm">
  <div class="card-body p-2 p-md-3">
    <div class="d-flex flex-row gap-3">
      <div class="flex-grow-1 d-flex flex-column justify-content-between">
        <div class="d-flex flex-column">
          <h5 class="employee-card-title mb-2 fw-semibold">Анципов Евгений Юрьевич</h5>
          <p class="mb-2"><strong class="text-primary"><i class="fa-solid fa-briefcase me-1"></i>Преподаватель</strong></p>
          <p class="mb-1"><small class="text-muted"><i class="fa-solid fa-award me-1"></i><strong>Квалификация:</strong></small> Второй категории</p>
          <p class="mb-1 text-justify"><small class="text-muted"><i class="fa-solid fa-graduation-cap me-1"></i><strong>Образование:</strong></small> Окончил БГУФК.</p>
        </div>
        <div class="d-flex flex-row align-items-center justify-content-between">
          <p class="mb-1"><small class="text-muted"><i class="fa-solid fa-clock me-1"></i><strong>Стаж:</strong></small> 7 лет</p>
        </div>
      </div>
    </div>
  </div>
</div>
`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(sampleHTML))
	if err != nil {
		t.Fatalf("failed reading sample html: %v", err)
	}

	parser := NewFacultyParser(slog.Default())
	members := parser.ParseHTML(doc, "teachers")

	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	m := members[0]
	if m.FullName != "Анципов Евгений Юрьевич" {
		t.Errorf("FullName = %q, want %q", m.FullName, "Анципов Евгений Юрьевич")
	}
	if m.ShortName != "Анципов Е. Ю." {
		t.Errorf("ShortName = %q, want %q", m.ShortName, "Анципов Е. Ю.")
	}
	if m.Position != "Преподаватель" {
		t.Errorf("Position = %q, want %q", m.Position, "Преподаватель")
	}
	if m.Qualification != "Второй категории" {
		t.Errorf("Qualification = %q, want %q", m.Qualification, "Второй категории")
	}
	if m.Education != "Окончил БГУФК." {
		t.Errorf("Education = %q, want %q", m.Education, "Окончил БГУФК.")
	}
	if m.Experience != "7 лет" {
		t.Errorf("Experience = %q, want %q", m.Experience, "7 лет")
	}
	if m.Department != "teachers" {
		t.Errorf("Department = %q, want %q", m.Department, "teachers")
	}
}
