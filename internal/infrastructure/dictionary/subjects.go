package dictionary

import (
	_ "embed"
	"encoding/csv"
	"maps"
	"regexp"
	"strings"
	"sync"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

//go:embed subjects.csv
var subjectsCSV []byte

var (
	subgroupPrefixRe = regexp.MustCompile(`^[123]\.\s*`)
	groupPrefixRe    = regexp.MustCompile(`^\d+\*?\s*[-–]\s*`)
)

type SubjectDictionary struct {
	mu       sync.RWMutex
	lookup   map[string]string // key: lowercase short_name -> full_name
	original map[string]string // key: original short_name -> full_name
}

func NewSubjectDictionary() *SubjectDictionary {
	d := &SubjectDictionary{
		lookup:   make(map[string]string),
		original: make(map[string]string),
	}
	d.loadEmbedded()
	return d
}

func (d *SubjectDictionary) loadEmbedded() {
	r := csv.NewReader(strings.NewReader(string(subjectsCSV)))
	records, err := r.ReadAll()
	if err != nil {
		return
	}

	for i, row := range records {
		if i == 0 || len(row) < 2 {
			continue // skip header
		}
		short := strings.TrimSpace(row[0])
		full := strings.TrimSpace(row[1])
		if short != "" && full != "" {
			d.original[short] = full
			d.lookup[strings.ToLower(short)] = full
		}
	}
}

// CleanName удаляет префиксы подгрупп (1. , 2. ) и групп в расписании преподавателей (107-, 169*-)
func CleanName(raw string) string {
	s := strings.TrimSpace(raw)
	s = subgroupPrefixRe.ReplaceAllString(s, "")
	s = groupPrefixRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// Lookup ищет полное наименование дисциплины по ее сокращенному названию
func (d *SubjectDictionary) Lookup(shortName string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	full, ok := d.lookup[strings.ToLower(strings.TrimSpace(shortName))]
	return full, ok
}

// Resolve очищает сырое название дисциплины от префиксов и возвращает очищенное имя и расшифровку
func (d *SubjectDictionary) Resolve(raw string) (cleaned string, fullName string, found bool) {
	cleaned = CleanName(raw)
	if cleaned == "" {
		return raw, "", false
	}

	fullName, found = d.Lookup(cleaned)
	return cleaned, fullName, found
}

// All возвращает копию словаря всех предметов
func (d *SubjectDictionary) All() map[string]string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	res := make(map[string]string, len(d.original))
	maps.Copy(res, d.original)
	return res
}

// Search выполняет поиск по сокращенному или полному названию
func (d *SubjectDictionary) Search(query string) []entities.SubjectInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	var res []entities.SubjectInfo

	for short, full := range d.original {
		if strings.Contains(strings.ToLower(short), q) || strings.Contains(strings.ToLower(full), q) {
			res = append(res, entities.SubjectInfo{
				ShortName: short,
				FullName:  full,
			})
		}
	}

	return res
}
