package entities

import "time"

type ChangeType int

const (
	ChangeTypeModified ChangeType = iota // Замена пары/аудитории/преподавателя/подгруппы
	ChangeTypeAdded                      // Появилась пара вместо окна/пустоты
	ChangeTypeRemoved                    // Пара отменена (окно)
)

type SubgroupDiff struct {
	Subgroup string     `json:"subgroup,omitempty"` // "1 п/г", "2 п/г" или "" (вся группа)
	Type     ChangeType `json:"type"`
	OldPair  *Pair      `json:"old_pair,omitempty"` // Было (nil для Added)
	NewPair  *Pair      `json:"new_pair,omitempty"` // Стало (nil для Removed)
}

type PairDiff struct {
	Number        int            `json:"number"`         // Номер пары (1-based: 1..6)
	SubgroupDiffs []SubgroupDiff `json:"subgroup_diffs"` // Точечные изменения по подгруппам (Вариант А)
}

type DayDiff struct {
	DayName  string     `json:"day_name"`  // Название дня ("Понедельник", "Вторник", ...)
	DayIndex int        `json:"day_index"` // 0 = Пн, 5 = Сб
	Date     time.Time  `json:"date"`      // Календарная дата конкретного дня (Week.AddDate(0, 0, DayIndex))
	Pairs    []PairDiff `json:"pairs"`     // Список изменений по парам в этот день
}

type ScheduleDiff struct {
	Title string    `json:"title"` // "Группа 101" или "Иванов И.И."
	Week  time.Time `json:"week"`  // Понедельник недели
	Days  []DayDiff `json:"days"`  // Дни с актуальными изменениями
}
