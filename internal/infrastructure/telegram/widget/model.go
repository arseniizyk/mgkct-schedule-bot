package widget

import "time"

// LessonItem represents a single lesson entry inside the widget.
type LessonItem struct {
	Num      int    `json:"n"`
	Subject  string `json:"s"`
	Type     string `json:"t,omitempty"`
	Room     string `json:"r,omitempty"`
	Teacher  string `json:"th,omitempty"`
	Group    string `json:"g,omitempty"`
	Subgroup int    `json:"sg,omitempty"` // 0: all, 1: 1st, 2: 2nd
}

// WidgetData holds all payload information embedded into the Mini-HTML widget.
type WidgetData struct {
	Title     string         `json:"t"`
	IsGroup   bool           `json:"g,omitempty"`
	WeekStart string         `json:"ws"`
	WeekEnd   string         `json:"we"`
	ActiveDay int            `json:"ad"`
	Days      [][]LessonItem `json:"d"`
}

// CallSlot represents bell timing.
type CallSlot struct {
	Num   int    `json:"num"`
	Time  string `json:"time"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// StandardBellSlots provides default call timings for MGKCT.
var StandardBellSlots = []struct {
	Num   int
	Start string
	End   string
	Text  string
	SMin  int
	EMin  int
}{
	{Num: 1, Start: "09:00", End: "10:40", Text: "09:00 - 09:45 | 09:55 - 10:40", SMin: 540, EMin: 640},
	{Num: 2, Start: "10:50", End: "12:40", Text: "10:50 - 11:35 | 11:55 - 12:40", SMin: 650, EMin: 760},
	{Num: 3, Start: "13:00", End: "14:40", Text: "13:00 - 13:45 | 13:55 - 14:40", SMin: 780, EMin: 880},
	{Num: 4, Start: "14:50", End: "16:30", Text: "14:50 - 15:35 | 15:45 - 16:30", SMin: 890, EMin: 990},
	{Num: 5, Start: "16:40", End: "18:20", Text: "16:40 - 17:25 | 17:35 - 18:20", SMin: 1000, EMin: 1100},
	{Num: 6, Start: "18:30", End: "20:10", Text: "18:30 - 19:15 | 19:25 - 20:10", SMin: 1110, EMin: 1210},
}

// PairTimeSlot returns the formatted time string and start/end minutes for a pair index (0-based).
func PairTimeSlot(pairIdx int) (timeStr string, startMin, endMin int) {
	if pairIdx >= 0 && pairIdx < len(StandardBellSlots) {
		slot := StandardBellSlots[pairIdx]
		return slot.Start + " - " + slot.End, slot.SMin, slot.EMin
	}
	return "—", 0, 0
}

// WeekDayNames maps index 0..5 to Russian day names and short codes.
var WeekDayNames = []struct {
	Full  string
	Short string
}{
	{Full: "Понедельник", Short: "Пн"},
	{Full: "Вторник", Short: "Вт"},
	{Full: "Среда", Short: "Ср"},
	{Full: "Четверг", Short: "Чт"},
	{Full: "Пятница", Short: "Пт"},
	{Full: "Суббота", Short: "Сб"},
}

// CalculateActiveDay returns today's weekday index (0=Mon .. 5=Sat).
// If Sunday or out of bounds, returns 0 (Monday).
func CalculateActiveDay(now time.Time) int {
	w := (int(now.Weekday()) + 6) % 7 // Mon=0..Sun=6
	if w >= 6 {
		return 0
	}
	return w
}
