package widget

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/dictionary"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

// BuildGroupWidget generates self-contained Mini-HTML for a group's weekly schedule.
func BuildGroupWidget(group *entities.Group, nowOpt ...time.Time) (string, error) {
	data, err := prepareGroupWidgetData(group, nowOpt...)
	if err != nil {
		return "", err
	}
	return RenderWidgetHTML(data)
}

func prepareGroupWidgetData(group *entities.Group, nowOpt ...time.Time) (WidgetData, error) {
	if group == nil {
		return WidgetData{}, fmt.Errorf("widget: group is nil")
	}

	now := timeutil.Now()
	if len(nowOpt) > 0 && !nowOpt[0].IsZero() {
		now = nowOpt[0].In(timeutil.MinskLocation)
	}

	monday := group.Week.In(timeutil.MinskLocation)
	saturday := monday.AddDate(0, 0, 5)

	data := WidgetData{
		Title:     fmt.Sprintf("Группа %d", group.ID),
		IsGroup:   true,
		WeekStart: monday.Format("02.01"),
		WeekEnd:   saturday.Format("02.01"),
		ActiveDay: CalculateActiveDay(now),
		Days:      make([][]LessonItem, 6),
	}

	for i := range 6 {
		lessons := make([]LessonItem, 0)

		if i < len(group.Days) && group.Days[i] != nil {
			day := group.Days[i]
			for pIdx, subj := range day.Subjects {
				if subj == nil || subj.IsEmpty || len(subj.Pairs) == 0 {
					continue
				}
				for _, p := range subj.Pairs {
					if p == nil {
						continue
					}
					name := dictionary.CleanName(p.Name)
					if name == "" && p.Group == "" {
						continue
					}
					sg := formatter.ParsePairSubgroup(p)
					teacherOrGroup := strings.TrimSpace(p.Group)
					room := strings.TrimPrefix(strings.TrimSpace(p.Class), "каб. ")
					if room == "-" {
						room = ""
					}
					lessons = append(lessons, LessonItem{
						Num:      pIdx + 1,
						Subject:  name,
						Type:     strings.TrimSpace(p.Type),
						Room:     room,
						Teacher:  teacherOrGroup,
						Subgroup: sg,
					})
				}
			}
		}

		data.Days[i] = lessons
	}

	return data, nil
}

// BuildTeacherWidget generates self-contained Mini-HTML for a teacher's weekly schedule.
func BuildTeacherWidget(teacher *entities.Teacher, nowOpt ...time.Time) (string, error) {
	data, err := prepareTeacherWidgetData(teacher, nowOpt...)
	if err != nil {
		return "", err
	}
	return RenderWidgetHTML(data)
}

func prepareTeacherWidgetData(teacher *entities.Teacher, nowOpt ...time.Time) (WidgetData, error) {
	if teacher == nil {
		return WidgetData{}, fmt.Errorf("widget: teacher is nil")
	}

	now := timeutil.Now()
	if len(nowOpt) > 0 && !nowOpt[0].IsZero() {
		now = nowOpt[0].In(timeutil.MinskLocation)
	}

	monday := teacher.Week.In(timeutil.MinskLocation)
	saturday := monday.AddDate(0, 0, 5)

	data := WidgetData{
		Title:     teacher.Name,
		IsGroup:   false,
		WeekStart: monday.Format("02.01"),
		WeekEnd:   saturday.Format("02.01"),
		ActiveDay: CalculateActiveDay(now),
		Days:      make([][]LessonItem, 6),
	}

	for i := range 6 {
		lessons := make([]LessonItem, 0)

		if i < len(teacher.Days) && teacher.Days[i] != nil {
			day := teacher.Days[i]
			for pIdx, subj := range day.Subjects {
				if subj == nil || subj.IsEmpty || len(subj.Pairs) == 0 {
					continue
				}
				for _, p := range subj.Pairs {
					if p == nil {
						continue
					}
					name := dictionary.CleanName(p.Name)
					if name == "" && p.Group == "" {
						continue
					}
					room := strings.TrimPrefix(strings.TrimSpace(p.Class), "каб. ")
					if room == "-" {
						room = ""
					}
					lessons = append(lessons, LessonItem{
						Num:      pIdx + 1,
						Subject:  name,
						Type:     strings.TrimSpace(p.Type),
						Room:     room,
						Group:    strings.TrimSpace(p.Group),
						Subgroup: 0,
					})
				}
			}
		}

		data.Days[i] = lessons
	}

	return data, nil
}

// RenderWidgetHTML formats the widget data into a compact Mini-HTML document.
func RenderWidgetHTML(data WidgetData) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("widget: marshal payload: %w", err)
	}

	html := fmt.Sprintf(widgetHTMLTemplate, string(jsonData))
	return html, nil
}
