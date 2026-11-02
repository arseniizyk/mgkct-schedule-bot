package renderer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

func TestRenderGroupWeek(t *testing.T) {
	group := &entities.Group{
		ID:   80,
		Week: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		Days: []*entities.Day{
			{
				Name: "Понедельник | 02.11.2026",
				Subjects: []*entities.Subject{
					{IsEmpty: true},
					{
						Pairs: []*entities.Pair{
							{Name: "1. НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "Павлов В. С."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "Основы права", Type: "Лек", Class: "2-212", Group: "Орлова Ю. Б."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "1. Осн эконом и ПД", Type: "ЛР", Class: "2-108", Group: "Васильева О. Н."},
							{Name: "2. НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "Павлов В. С."},
						},
					},
				},
			},
			{
				Name: "Вторник | 03.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "1. НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "Павлов В. С."},
							{Name: "2. ИнЯз Проф", Type: "ЛР", Class: "3-210", Group: "Макарова Ю. В."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "Техоб и ср роб", Type: "Лек", Class: "2-211", Group: "Степанов А. А."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "Техоб и ср роб", Type: "Лек", Class: "2-211", Group: "Степанов А. А."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "1. ИнЯз Проф", Type: "ЛР", Class: "-", Group: "Попова Д. Ч."},
							{Name: "2. НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "Павлов В. С."},
						},
					},
				},
			},
			{
				Name: "Среда | 04.11.2026",
				Subjects: []*entities.Subject{
					{IsEmpty: true},
					{
						Pairs: []*entities.Pair{
							{Name: "1. Осн эконом и ПД", Type: "ЛР", Class: "2-108", Group: "Васильева О. Н."},
							{Name: "2. ИнЯз Проф", Type: "ЛР", Class: "3-210", Group: "Макарова Ю. В."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "Техоб и ср роб", Type: "Лек", Class: "-", Group: "Степанов А. А."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "Осн эконом и ПД", Type: "Лек", Class: "2-108", Group: "Васильева О. Н."},
						},
					},
				},
			},
			{
				Name: "Четверг | 05.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "1. Пром контрол", Type: "ЛР", Class: "Кн.2-413 маст", Group: "Соколов Н. Д."},
							{Name: "2. Осн эконом и ПД", Type: "ЛР", Class: "-", Group: "Васильева О. Н."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "1. Пром контрол", Type: "ЛР", Class: "Кн.2-413 маст", Group: "Соколов Н. Д."},
							{Name: "2. Техоб и ср роб", Type: "ЛР", Class: "-", Group: "Степанов А. А."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "1. Техоб и ср роб", Type: "ЛР", Class: "-", Group: "Степанов А. А."},
							{Name: "2. Пром контрол", Type: "ЛР", Class: "Кн.2-413 маст", Group: "Соколов Н. Д."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "1. Техоб и ср роб", Type: "ЛР", Class: "-", Group: "Степанов А. А."},
							{Name: "2. Пром контрол", Type: "ЛР", Class: "Кн.2-413 маст", Group: "Соколов Н. Д."},
						},
					},
				},
			},
			{
				Name: "Пятница | 06.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "2. НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "Павлов В. С."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "1. ФЗКиЗ", Type: "ЛР", Class: "-", Group: "Смирнов С. С."},
							{Name: "2. ФЗКиЗ", Type: "ЛР", Class: "-", Group: "Николаева Л. Н."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "1. НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "Павлов В. С."},
							{Name: "2. ИнЯз Проф", Type: "ЛР", Class: "3-210", Group: "Макарова Ю. В."},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "Осн эконом и ПД", Type: "Лек", Class: "2-108", Group: "Васильева О. Н."},
						},
					},
				},
			},
			{
				Name: "Суббота | 07.11.2026",
				Subjects: []*entities.Subject{
					{IsEmpty: true},
					{IsEmpty: true},
					{IsEmpty: true},
					{IsEmpty: true},
				},
			},
		},
	}

	pngBytes, err := RenderGroupWeek(group, 0)
	if err != nil {
		t.Fatalf("RenderGroupWeek failed: %v", err)
	}

	if len(pngBytes) == 0 {
		t.Fatal("RenderGroupWeek returned empty bytes")
	}

	// Проверяем валидность сигнатуры PNG (\x89PNG\r\n\x1a\n)
	pngMagic := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if len(pngBytes) < 8 || string(pngBytes[:8]) != string(pngMagic) {
		t.Fatalf("Invalid PNG header: %v", pngBytes[:8])
	}

	// Тест рендеринга для конкретной подгруппы (заголовок с подгруппой)
	pngBytesSg, err := RenderGroupWeek(group, 1)
	if err != nil {
		t.Fatalf("RenderGroupWeek subgroup 1 failed: %v", err)
	}
	if len(pngBytesSg) < 8 || string(pngBytesSg[:8]) != string(pngMagic) {
		t.Fatalf("Invalid PNG header for subgroup: %v", pngBytesSg[:8])
	}

	outPath := filepath.Join(t.TempDir(), "schedule_pure_go_rendered.png")
	if err := os.WriteFile(outPath, pngBytes, 0644); err != nil {
		t.Fatalf("Failed to save image: %v", err)
	}
}

func TestRenderTeacherWeek(t *testing.T) {
	teacher := &entities.Teacher{
		Name: "Павлов В. С.",
		Week: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		Days: []*entities.Day{
			{
				Name: "Понедельник | 02.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "80"},
						},
					},
					{
						Pairs: []*entities.Pair{
							{Name: "НЭОПИМ", Type: "Лек", Class: "3-114", Group: "88"},
						},
					},
				},
			},
			{
				Name: "Вторник | 03.11.2026",
				Subjects: []*entities.Subject{
					{IsEmpty: true},
					{
						Pairs: []*entities.Pair{
							{Name: "НЭОПИМ", Type: "ЛР", Class: "3-114", Group: "80"},
						},
					},
				},
			},
		},
	}

	pngBytes, err := RenderTeacherWeek(teacher)
	if err != nil {
		t.Fatalf("RenderTeacherWeek failed: %v", err)
	}

	pngMagic := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if len(pngBytes) < 8 || string(pngBytes[:8]) != string(pngMagic) {
		t.Fatalf("Invalid PNG header: %v", pngBytes[:8])
	}

	// Nil teacher
	if _, err := RenderTeacherWeek(nil); err == nil {
		t.Error("expected error for nil teacher, got nil")
	}
}

func TestFindLastLessonIndex(t *testing.T) {
	// Nil day
	if idx := findLastLessonIndex(nil); idx != -1 {
		t.Errorf("expected -1 for nil day, got %d", idx)
	}

	// Empty subjects
	if idx := findLastLessonIndex(&entities.Day{}); idx != -1 {
		t.Errorf("expected -1 for empty day, got %d", idx)
	}

	// 7 subjects with only pairs 1..4 non-empty (standard college schedule from scraper)
	day4 := &entities.Day{
		Subjects: []*entities.Subject{
			{Pairs: []*entities.Pair{{Name: "Предмет 1", Group: "Препод"}}},
			{Pairs: []*entities.Pair{{Name: "Предмет 2", Group: "Препод"}}},
			{Pairs: []*entities.Pair{{Name: "Предмет 3", Group: "Препод"}}},
			{Pairs: []*entities.Pair{{Name: "Предмет 4", Group: "Препод"}}},
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
		},
	}
	if idx := findLastLessonIndex(day4); idx != 3 {
		t.Errorf("expected 3 (4th pair), got %d", idx)
	}

	// 7 subjects with pair 5 non-empty
	day5 := &entities.Day{
		Subjects: []*entities.Subject{
			{Pairs: []*entities.Pair{{Name: "Предмет 1", Group: "Препод"}}},
			{IsEmpty: true},
			{Pairs: []*entities.Pair{{Name: "Предмет 3", Group: "Препод"}}},
			{IsEmpty: true},
			{Pairs: []*entities.Pair{{Name: "Предмет 5", Group: "Препод"}}},
			{IsEmpty: true},
			{IsEmpty: true},
		},
	}
	if idx := findLastLessonIndex(day5); idx != 4 {
		t.Errorf("expected 4 (5th pair), got %d", idx)
	}

	// 7 subjects all empty (Saturday)
	daySat := &entities.Day{
		Subjects: []*entities.Subject{
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
			{IsEmpty: true},
		},
	}
	if idx := findLastLessonIndex(daySat); idx != -1 {
		t.Errorf("expected -1 for empty Saturday, got %d", idx)
	}
}

func TestRenderGroupWeek_ConcurrencyAndSingleflight(t *testing.T) {
	group := &entities.Group{
		ID:   99,
		Week: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		Days: []*entities.Day{
			{
				Name: "Понедельник | 02.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Тест", Type: "Лек", Class: "101", Group: "Препод"},
						},
					},
				},
			},
		},
	}

	const goroutines = 10
	errCh := make(chan error, goroutines)

	for range goroutines {
		go func() {
			bytes, err := RenderGroupWeek(group, 0)
			if err != nil {
				errCh <- err
				return
			}
			if len(bytes) == 0 {
				errCh <- os.ErrInvalid
				return
			}
			errCh <- nil
		}()
	}

	for range goroutines {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent render failed: %v", err)
		}
	}
}

func TestRender_DifferentConcurrentRenders(t *testing.T) {
	const goroutines = 6
	errCh := make(chan error, goroutines)

	for i := range goroutines {
		grp := &entities.Group{
			ID:   int32(100 + i),
			Week: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
			Days: []*entities.Day{
				{
					Name: "Понедельник | 02.11.2026",
					Subjects: []*entities.Subject{
						{
							Pairs: []*entities.Pair{
								{Name: "Пара", Type: "Лек", Class: "101", Group: "Преподаватель"},
							},
						},
					},
				},
			},
		}

		go func(g *entities.Group) {
			b, err := RenderGroupWeek(g, 0)
			if err != nil {
				errCh <- err
				return
			}
			if len(b) == 0 {
				errCh <- os.ErrInvalid
				return
			}
			errCh <- nil
		}(grp)
	}

	for range goroutines {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent different renders failed: %v", err)
		}
	}
}
