package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

func TestComputeGroupDiff_ModifiedPair(t *testing.T) {
	week := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC) // future week

	oldGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Информатика", Class: "305", Group: "2 п/г"},
						},
					},
				},
			},
		},
	}

	newGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Информатика", Class: "102", Group: "2 п/г"},
						},
					},
				},
			},
		},
	}

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	diff := service.ComputeGroupDiff(oldGroup, newGroup, now)

	require.NotNil(t, diff)
	assert.Equal(t, "Группа 101", diff.Title)
	require.Len(t, diff.Days, 1)

	dayDiff := diff.Days[0]
	assert.Equal(t, "Понедельник", dayDiff.DayName)
	assert.Equal(t, 0, dayDiff.DayIndex)
	require.Len(t, dayDiff.Pairs, 1)

	pairDiff := dayDiff.Pairs[0]
	assert.Equal(t, 1, pairDiff.Number)
	require.Len(t, pairDiff.SubgroupDiffs, 1)

	sgDiff := pairDiff.SubgroupDiffs[0]
	assert.Equal(t, "2 п/г", sgDiff.Subgroup)
	assert.Equal(t, entities.ChangeTypeModified, sgDiff.Type)
	assert.Equal(t, "305", sgDiff.OldPair.Class)
	assert.Equal(t, "102", sgDiff.NewPair.Class)
}

func TestComputeGroupDiff_AddedAndRemovedPair(t *testing.T) {
	week := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	oldGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Вторник",
				Subjects: []*entities.Subject{
					{IsEmpty: true}, // пара 1: окно
					{
						Pairs: []*entities.Pair{
							{Name: "Химия", Class: "110"},
						},
					}, // пара 2: Химия
				},
			},
		},
	}

	newGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Вторник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Информатика", Type: "Лаб", Class: "102"},
						},
					}, // пара 1: добавилась
					{IsEmpty: true}, // пара 2: окно (отменена)
				},
			},
		},
	}

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	diff := service.ComputeGroupDiff(oldGroup, newGroup, now)

	require.NotNil(t, diff)
	require.Len(t, diff.Days, 1)
	require.Len(t, diff.Days[0].Pairs, 2)

	p1 := diff.Days[0].Pairs[0]
	assert.Equal(t, 1, p1.Number)
	require.Len(t, p1.SubgroupDiffs, 1)
	assert.Equal(t, entities.ChangeTypeAdded, p1.SubgroupDiffs[0].Type)
	assert.Equal(t, "Информатика", p1.SubgroupDiffs[0].NewPair.Name)

	p2 := diff.Days[0].Pairs[1]
	assert.Equal(t, 2, p2.Number)
	require.Len(t, p2.SubgroupDiffs, 1)
	assert.Equal(t, entities.ChangeTypeRemoved, p2.SubgroupDiffs[0].Type)
	assert.Equal(t, "Химия", p2.SubgroupDiffs[0].OldPair.Name)
}

func TestComputeGroupDiff_SubgroupsGranularChange(t *testing.T) {
	week := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	// Обе подгруппы были. 1-я подгруппа НЕ меняется, 2-я подгруппа меняет кабинет.
	oldGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Среда",
				Subjects: []*entities.Subject{
					nil,
					{
						Pairs: []*entities.Pair{
							{Name: "Информатика", Class: "101", Group: "1 п/г"},
							{Name: "Информатика", Class: "305", Group: "2 п/г"},
						},
					},
				},
			},
		},
	}

	newGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Среда",
				Subjects: []*entities.Subject{
					nil,
					{
						Pairs: []*entities.Pair{
							{Name: "Информатика", Class: "101", Group: "1 п/г"},
							{Name: "Информатика", Class: "102", Group: "2 п/г"},
						},
					},
				},
			},
		},
	}

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	diff := service.ComputeGroupDiff(oldGroup, newGroup, now)

	require.NotNil(t, diff)
	require.Len(t, diff.Days, 1)
	require.Len(t, diff.Days[0].Pairs, 1)

	pairDiff := diff.Days[0].Pairs[0]
	assert.Equal(t, 2, pairDiff.Number)

	// Должно быть точечное изменение ТОЛЬКО для 2-й подгруппы!
	require.Len(t, pairDiff.SubgroupDiffs, 1)
	assert.Equal(t, "2 п/г", pairDiff.SubgroupDiffs[0].Subgroup)
	assert.Equal(t, entities.ChangeTypeModified, pairDiff.SubgroupDiffs[0].Type)
	assert.Equal(t, "305", pairDiff.SubgroupDiffs[0].OldPair.Class)
	assert.Equal(t, "102", pairDiff.SubgroupDiffs[0].NewPair.Class)
}

func TestComputeGroupDiff_GeneralToSubgroups(t *testing.T) {
	week := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	// Была общая пара, стала разбита на подгруппы
	oldGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Физика", Class: "201"},
						},
					},
				},
			},
		},
	}

	newGroup := &entities.Group{
		ID:   101,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Физика", Class: "201", Group: "1 п/г"},
							{Name: "Информатика", Class: "102", Group: "2 п/г"},
						},
					},
				},
			},
		},
	}

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	diff := service.ComputeGroupDiff(oldGroup, newGroup, now)

	require.NotNil(t, diff)
	require.Len(t, diff.Days, 1)
	require.Len(t, diff.Days[0].Pairs, 1)

	// Должно быть снятие общей пары и добавление двух подгрупп
	sgs := diff.Days[0].Pairs[0].SubgroupDiffs
	require.Len(t, sgs, 3)
	assert.Equal(t, entities.ChangeTypeRemoved, sgs[0].Type)
	assert.Empty(t, sgs[0].Subgroup)
	assert.Equal(t, entities.ChangeTypeAdded, sgs[1].Type)
	assert.Equal(t, "1 п/г", sgs[1].Subgroup)
	assert.Equal(t, entities.ChangeTypeAdded, sgs[2].Type)
	assert.Equal(t, "2 п/г", sgs[2].Subgroup)
}

func TestComputeGroupDiff_BellScheduleAndPastDaysFiltering(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)
	// Текущая неделя начинается в Пн 12 октября 2026
	currentWeekMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	// Сейчас Среда, 14 октября 2026, 11:30 (Минск)
	// В среду:
	// Пара 1 (09:00): уже началась (11:30 >= 09:00) -> должна игнорироваться!
	// Пара 2 (10:50): уже началась (11:30 >= 10:50) -> должна игнорироваться!
	// Пара 3 (13:00): еще предстоит (11:30 < 13:00) -> должна быть включена!
	now := time.Date(2026, 10, 14, 11, 30, 0, 0, minskLoc)

	oldGroup := &entities.Group{
		ID:   101,
		Week: currentWeekMonday,
		Days: []*entities.Day{
			// Дни 0 (Пн) и 1 (Вт) — строго в прошлом относительно среды (14 октября)
			{Name: "Понедельник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Старая"}}}, nil}},
			{Name: "Вторник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Старая"}}}, nil}},
			// День 2 (Ср) — сегодня!
			{
				Name: "Среда",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Пара 1 Старая"}}}, // 09:00
					{Pairs: []*entities.Pair{{Name: "Пара 2 Старая"}}}, // 10:50
					{Pairs: []*entities.Pair{{Name: "Пара 3 Старая"}}}, // 13:00
				},
			},
			// День 3 (Чт) — завтра (в будущем)
			{Name: "Четверг", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Старая Чт"}}}}},
		},
	}

	newGroup := &entities.Group{
		ID:   101,
		Week: currentWeekMonday,
		Days: []*entities.Day{
			// Изменения задним числом в Пн и Вт
			{Name: "Понедельник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Новая"}}}, nil}},
			{Name: "Вторник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Новая"}}}, nil}},
			// Изменения сегодня в Ср
			{
				Name: "Среда",
				Subjects: []*entities.Subject{
					{Pairs: []*entities.Pair{{Name: "Пара 1 Новая"}}}, // звонок прошел
					{Pairs: []*entities.Pair{{Name: "Пара 2 Новая"}}}, // звонок прошел
					{Pairs: []*entities.Pair{{Name: "Пара 3 Новая"}}}, // еще предстоит!
				},
			},
			// Изменения завтра в Чт
			{Name: "Четверг", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Новая Чт"}}}}},
		},
	}

	diff := service.ComputeGroupDiff(oldGroup, newGroup, now)
	require.NotNil(t, diff)

	// Пн и Вт должны быть полностью отфильтрованы!
	// Остались только Среда (день 2) и Четверг (день 3)
	require.Len(t, diff.Days, 2)

	wed := diff.Days[0]
	assert.Equal(t, 2, wed.DayIndex)
	assert.Equal(t, "Среда", wed.DayName)
	// В среду должна остаться ТОЛЬКО Пара 3 (Пары 1 и 2 уже начались)
	require.Len(t, wed.Pairs, 1)
	assert.Equal(t, 3, wed.Pairs[0].Number)
	assert.Equal(t, "Пара 3 Новая", wed.Pairs[0].SubgroupDiffs[0].NewPair.Name)

	thu := diff.Days[1]
	assert.Equal(t, 3, thu.DayIndex)
	assert.Equal(t, "Четверг", thu.DayName)
	require.Len(t, thu.Pairs, 1)
	assert.Equal(t, 1, thu.Pairs[0].Number)
	assert.Equal(t, "Новая Чт", thu.Pairs[0].SubgroupDiffs[0].NewPair.Name)
}

func TestComputeGroupDiff_PastWeekReturnsNil(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, minskLoc) // Неделя 12.10.2026

	// Прошедшая неделя: 05.10.2026
	pastWeek := time.Date(2026, 10, 5, 0, 0, 0, 0, minskLoc)

	oldGroup := &entities.Group{
		ID:   101,
		Week: pastWeek,
		Days: []*entities.Day{
			{Name: "Понедельник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "A"}}}}},
		},
	}

	newGroup := &entities.Group{
		ID:   101,
		Week: pastWeek,
		Days: []*entities.Day{
			{Name: "Понедельник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "B"}}}}},
		},
	}

	diff := service.ComputeGroupDiff(oldGroup, newGroup, now)
	assert.Nil(t, diff, "Изменения за прошедшие недели не должны формировать уведомление diff")
}

func TestComputeTeacherDiff_AllCases(t *testing.T) {
	week := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	oldTeacher := &entities.Teacher{
		Name: "Иванов И.И.",
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Среда",
				Subjects: []*entities.Subject{
					// Пара 1: замена
					{Pairs: []*entities.Pair{{Group: "101", Name: "Математика", Class: "204"}}},
					// Пара 2: без изменений
					{Pairs: []*entities.Pair{{Group: "101", Name: "Геометрия", Class: "204"}}},
					// Пара 3: отменена
					{Pairs: []*entities.Pair{{Group: "201", Name: "Высшая математика", Class: "105"}}},
					// Пара 4: окно -> новая пара
					{IsEmpty: true},
				},
			},
		},
	}

	newTeacher := &entities.Teacher{
		Name: "Иванов И.И.",
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Среда",
				Subjects: []*entities.Subject{
					// Пара 1: 102 | Математика | ауд. 301
					{Pairs: []*entities.Pair{{Group: "102", Name: "Математика", Class: "301"}}},
					// Пара 2: без изменений
					{Pairs: []*entities.Pair{{Group: "101", Name: "Геометрия", Class: "204"}}},
					// Пара 3: окно (отменена)
					{IsEmpty: true},
					// Пара 4: появилась пара
					{Pairs: []*entities.Pair{{Group: "301", Name: "Алгебра", Class: "205"}}},
				},
			},
		},
	}

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	diff := service.ComputeTeacherDiff(oldTeacher, newTeacher, now)

	require.NotNil(t, diff)
	assert.Equal(t, "Иванов И.И.", diff.Title)
	require.Len(t, diff.Days, 1)

	pairs := diff.Days[0].Pairs
	require.Len(t, pairs, 3)

	// Пара 1: Modified
	assert.Equal(t, 1, pairs[0].Number)
	assert.Equal(t, entities.ChangeTypeModified, pairs[0].SubgroupDiffs[0].Type)
	assert.Equal(t, "101", pairs[0].SubgroupDiffs[0].OldPair.Group)
	assert.Equal(t, "102", pairs[0].SubgroupDiffs[0].NewPair.Group)

	// Пара 3: Removed
	assert.Equal(t, 3, pairs[1].Number)
	assert.Equal(t, entities.ChangeTypeRemoved, pairs[1].SubgroupDiffs[0].Type)
	assert.Equal(t, "201", pairs[1].SubgroupDiffs[0].OldPair.Group)

	// Пара 4: Added
	assert.Equal(t, 4, pairs[2].Number)
	assert.Equal(t, entities.ChangeTypeAdded, pairs[2].SubgroupDiffs[0].Type)
	assert.Equal(t, "301", pairs[2].SubgroupDiffs[0].NewPair.Group)
}
