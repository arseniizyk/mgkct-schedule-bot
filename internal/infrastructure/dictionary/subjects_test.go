package dictionary

import (
	"testing"
)

func TestSubjectDictionary_Resolve(t *testing.T) {
	d := NewSubjectDictionary()

	tests := []struct {
		name         string
		raw          string
		wantCleaned  string
		wantFullName string
		wantFound    bool
	}{
		{
			name:         "exact match abbreviation",
			raw:          "ТОЭ",
			wantCleaned:  "ТОЭ",
			wantFullName: "Теоретические основы электротехники",
			wantFound:    true,
		},
		{
			name:         "subgroup prefix 1.",
			raw:          "1. Ин Яз",
			wantCleaned:  "Ин Яз",
			wantFullName: "Иностранный язык",
			wantFound:    true,
		},
		{
			name:         "subgroup prefix 2.",
			raw:          "2. Матем в професс",
			wantCleaned:  "Матем в професс",
			wantFullName: "Математика в профессиональной деятельности",
			wantFound:    true,
		},
		{
			name:         "teacher schedule group prefix 107-",
			raw:          "107-Бел Яз",
			wantCleaned:  "Бел Яз",
			wantFullName: "Белорусский язык",
			wantFound:    true,
		},
		{
			name:         "teacher schedule group prefix with star 169*-",
			raw:          "169*-КПиЯП",
			wantCleaned:  "КПиЯП",
			wantFullName: "Конструирование программ и языки программирования",
			wantFound:    true,
		},
		{
			name:         "trpo abbreviation",
			raw:          "ТРПО",
			wantCleaned:  "ТРПО",
			wantFullName: "Технология разработки программного обеспечения",
			wantFound:    true,
		},
		{
			name:         "trpo with subgroup prefix",
			raw:          "1. ТРПО",
			wantCleaned:  "ТРПО",
			wantFullName: "Технология разработки программного обеспечения",
			wantFound:    true,
		},
		{
			name:         "ipo abbreviation",
			raw:          "ИПО",
			wantCleaned:  "ИПО",
			wantFullName: "Инструментальные средства разработки программного обеспечения",
			wantFound:    true,
		},
		{
			name:         "ipo with subgroup prefix",
			raw:          "2. ИПО",
			wantCleaned:  "ИПО",
			wantFullName: "Инструментальные средства разработки программного обеспечения",
			wantFound:    true,
		},
		{
			name:         "alovt abbreviation",
			raw:          "АЛОВТ",
			wantCleaned:  "АЛОВТ",
			wantFullName: "Арифметико-логические основы вычислительной техники",
			wantFound:    true,
		},
		{
			name:         "alovt with subgroup prefix",
			raw:          "1. АЛОВТ",
			wantCleaned:  "АЛОВТ",
			wantFullName: "Арифметико-логические основы вычислительной техники",
			wantFound:    true,
		},
		{
			name:         "subd abbreviation",
			raw:          "СУБД",
			wantCleaned:  "СУБД",
			wantFullName: "Системы управления базами данных (СУБД)",
			wantFound:    true,
		},
		{
			name:         "os abbreviation",
			raw:          "ОС",
			wantCleaned:  "ОС",
			wantFullName: "Операционные системы",
			wantFound:    true,
		},
		{
			name:         "oaip abbreviation",
			raw:          "ОАиП",
			wantCleaned:  "ОАиП",
			wantFullName: "Основы алгоритмизации и программирования",
			wantFound:    true,
		},
		{
			name:         "tpo abbreviation",
			raw:          "ТПО",
			wantCleaned:  "ТПО",
			wantFullName: "Тестирование программного обеспечения",
			wantFound:    true,
		},
		{
			name:         "deloproizvodstvo raw subject",
			raw:          "Делопроизводств",
			wantCleaned:  "Делопроизводств",
			wantFullName: "Делопроизводство",
			wantFound:    true,
		},
		{
			name:         "belyaz merged subject",
			raw:          "БелЯз",
			wantCleaned:  "БелЯз",
			wantFullName: "Белорусский язык",
			wantFound:    true,
		},
		{
			name:         "case-insensitive lookup",
			raw:          "трпо",
			wantCleaned:  "трпо",
			wantFullName: "Технология разработки программного обеспечения",
			wantFound:    true,
		},
		{
			name:         "unknown subject",
			raw:          "НеизвестныйПредмет123",
			wantCleaned:  "НеизвестныйПредмет123",
			wantFullName: "",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, full, found := d.Resolve(tt.raw)
			if cleaned != tt.wantCleaned {
				t.Errorf("cleaned = %q, want %q", cleaned, tt.wantCleaned)
			}
			if full != tt.wantFullName {
				t.Errorf("full = %q, want %q", full, tt.wantFullName)
			}
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
		})
	}
}

func TestSubjectDictionary_Search(t *testing.T) {
	d := NewSubjectDictionary()
	res := d.Search("электро")
	if len(res) == 0 {
		t.Errorf("expected search results for 'электро', got 0")
	}
}
