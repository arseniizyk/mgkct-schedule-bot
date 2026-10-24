package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type TeacherScheduleRepository interface {
	Save(ctx context.Context, name string, week time.Time, schedule *entities.Teacher) error
	GetByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	GetLatest(ctx context.Context, name string) (*entities.Teacher, error)
	GetAllLatest(ctx context.Context) (map[string]*entities.Teacher, error)
	GetAll(ctx context.Context) ([]*entities.Teacher, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	GetAllWeeks(ctx context.Context, name string) ([]time.Time, error)
	GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
}

type FacultyRepository interface {
	SaveBatch(ctx context.Context, members []*entities.FacultyMember) error
	GetAll(ctx context.Context) ([]*entities.FacultyMember, error)
	FindByShortName(ctx context.Context, shortName string) (*entities.FacultyMember, error)
	Search(ctx context.Context, queryStr string) ([]*entities.FacultyMember, error)
}

type SubjectDictionary interface {
	Lookup(shortName string) (string, bool)
	Resolve(raw string) (cleaned string, fullName string, found bool)
	All() map[string]string
	Search(query string) []entities.SubjectInfo
}

type TeacherService struct {
	log         *slog.Logger
	repo        TeacherScheduleRepository
	facultyRepo FacultyRepository
	dict        SubjectDictionary
	nowFunc     func() time.Time
}

func NewTeacherService(
	log *slog.Logger,
	repo TeacherScheduleRepository,
	facultyRepo FacultyRepository,
	dict SubjectDictionary,
) *TeacherService {
	return &TeacherService{
		log:         log,
		repo:        repo,
		facultyRepo: facultyRepo,
		dict:        dict,
	}
}

func (s *TeacherService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

func (s *TeacherService) SetNowFunc(fn func() time.Time) {
	s.nowFunc = fn
}

func (s *TeacherService) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	const op = "service.TeacherService.GetTeacherLatestSchedule"
	log := s.log.With("operation", op, "name", name)

	targetWeek := EffectiveScheduleWeek(s.now())
	t, err := s.repo.GetByWeek(ctx, name, targetWeek)
	if err == nil && t != nil {
		return t, nil
	}

	t, err = s.repo.GetLatest(ctx, name)
	if err != nil {
		if errors.Is(err, domainerr.ErrScheduleNotFound) {
			log.WarnContext(ctx, "teacher schedule not found in repository")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by latest", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return t, nil
}

func (s *TeacherService) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	const op = "service.TeacherService.GetTeacherScheduleByWeek"
	log := s.log.With("operation", op, "name", name, "week", week)

	t, err := s.repo.GetByWeek(ctx, name, week)
	if err != nil {
		if errors.Is(err, domainerr.ErrWeekNotFound) {
			log.WarnContext(ctx, "teacher week not found")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by week", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return t, nil
}

func (s *TeacherService) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return s.repo.GetAllTeacherNames(ctx)
}

func (s *TeacherService) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return s.repo.GetWeeks(ctx, name, week)
}

func (s *TeacherService) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	exact, partial, err := s.FindTeacherByName(ctx, name)
	if err != nil {
		s.log.ErrorContext(ctx, "failed finding teacher by name", "name", name, "error", err)
		return "", nil, false
	}

	// Если есть ровно одно точное совпадение — отдаем его сразу
	if len(exact) == 1 {
		return exact[0], nil, true
	}

	// Если точных нет, проверяем частичные совпадения
	if len(exact) == 0 {
		switch len(partial) {
		case 0:
			return "", nil, false
		case 1:
			return partial[0], nil, true
		default:
			return "", partial, false
		}
	}

	// Если почему-то точных совпадений > 1
	return "", exact, false
}

func (s *TeacherService) FindTeacherByName(ctx context.Context, name string) (exact, partial []string, err error) {
	names, err := s.repo.GetAllTeacherNames(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get all teacher names: %w", err)
	}

	lowerName := strings.ToLower(strings.TrimSpace(name))

	// 1. Прямой поиск по расписанию преподавателей
	for _, n := range names {
		lower := strings.ToLower(n)
		switch {
		case lower == lowerName:
			exact = append(exact, n)
		case strings.Contains(lower, lowerName):
			partial = append(partial, n)
		}
	}

	// 2. Если точных нет и подключен репозиторий педсостава, ищем по полному ФИО в справочнике
	if len(exact) == 0 && s.facultyRepo != nil {
		members, err := s.facultyRepo.Search(ctx, name)
		if err == nil {
			namesSet := make(map[string]struct{}, len(names))
			for _, n := range names {
				namesSet[n] = struct{}{}
			}

			for _, m := range members {
				if _, ok := namesSet[m.ShortName]; !ok {
					continue
				}
				if strings.EqualFold(m.FullName, strings.TrimSpace(name)) {
					if !slices.Contains(exact, m.ShortName) {
						exact = append(exact, m.ShortName)
					}
				} else {
					if !slices.Contains(partial, m.ShortName) && !slices.Contains(exact, m.ShortName) {
						partial = append(partial, m.ShortName)
					}
				}
			}
		}
	}

	return exact, partial, nil
}

func (s *TeacherService) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	if s.facultyRepo == nil {
		return nil, domainerr.ErrTeacherNotFound
	}
	return s.facultyRepo.FindByShortName(ctx, teacherName)
}

func (s *TeacherService) GetAllFaculty(ctx context.Context) ([]*entities.FacultyMember, error) {
	if s.facultyRepo == nil {
		return nil, domainerr.ErrTeacherNotFound
	}
	return s.facultyRepo.GetAll(ctx)
}

func (s *TeacherService) GetSubjectDictionary() SubjectDictionary {
	return s.dict
}

func (s *TeacherService) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	return s.repo.GetAllWeeks(ctx, name)
}

func (s *TeacherService) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	const op = "service.TeacherService.GetTeacherArchiveSchedule"

	targetDate = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.UTC)
	weekday := (int(targetDate.Weekday()) + 6) % 7 // Пн=0..Вс=6
	monday := targetDate.AddDate(0, 0, -weekday)

	t, err := s.GetTeacherScheduleByWeek(ctx, name, monday)
	if err == nil {
		return t, nil
	}

	t, err = s.repo.GetByWeek(ctx, name, targetDate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	diff := targetDate.Sub(t.Week)
	if diff < 0 || diff > 7*24*time.Hour {
		return nil, domainerr.ErrWeekNotFound
	}

	return t, nil
}

// FindSimilarTeachers ищет до limit преподавателей с похожими именами (при опечатках).
func (s *TeacherService) FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error) {
	const op = "service.TeacherService.FindSimilarTeachers"

	if limit <= 0 {
		limit = 3
	}

	rawQuery := strings.TrimSpace(name)
	normQuery := normalizeForComparison(rawQuery)
	queryRunes := []rune(normQuery)
	if len(queryRunes) < 3 {
		return nil, nil
	}

	names, err := s.repo.GetAllTeacherNames(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed getting teacher names for similarity search", "operation", op, "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var facultyMap map[string]*entities.FacultyMember
	if s.facultyRepo != nil {
		members, err := s.facultyRepo.GetAll(ctx)
		if err == nil {
			facultyMap = make(map[string]*entities.FacultyMember, len(members))
			for _, m := range members {
				facultyMap[m.ShortName] = m
			}
		}
	}

	layoutQuery := convertKeyboardLayout(normQuery)
	var layoutRunes []rune
	if layoutQuery != normQuery {
		layoutRunes = []rune(layoutQuery)
	}

	queryWords := strings.Fields(normQuery)

	var candidates []candidateMatch
	seen := make(map[string]struct{}, len(names))

	for _, teacherName := range names {
		if _, exists := seen[teacherName]; exists {
			continue
		}
		seen[teacherName] = struct{}{}

		var fm *entities.FacultyMember
		if facultyMap != nil {
			fm = facultyMap[teacherName]
		}

		if match, ok := calcTeacherMatch(teacherName, queryRunes, layoutRunes, queryWords, fm); ok {
			candidates = append(candidates, match)
		}
	}

	slices.SortFunc(candidates, func(a, b candidateMatch) int {
		if a.dist != b.dist {
			return a.dist - b.dist
		}
		if a.similarity != b.similarity {
			if a.similarity > b.similarity {
				return -1
			}
			return 1
		}
		return strings.Compare(a.name, b.name)
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]string, len(candidates))
	for i, c := range candidates {
		result[i] = c.name
	}

	return result, nil
}

type candidateMatch struct {
	name       string
	dist       int
	similarity float64
}

func calcTeacherMatch(
	teacherName string,
	queryRunes []rune,
	layoutRunes []rune,
	queryWords []string,
	fm *entities.FacultyMember,
) (candidateMatch, bool) {
	shortNorm := normalizeForComparison(teacherName)
	teacherParts := strings.Fields(shortNorm)
	lastName := ""
	if len(teacherParts) > 0 {
		lastName = teacherParts[0]
	}

	var fullName, facultyLastName string
	if fm != nil {
		fullName = normalizeForComparison(fm.FullName)
		fmParts := strings.Fields(fullName)
		if len(fmParts) > 0 {
			facultyLastName = fmParts[0]
		}
	}

	bestDist := 999
	bestSim := 0.0

	checkTarget := func(qRunes, tRunes []rune) {
		if len(tRunes) == 0 {
			return
		}
		d := damerauLevenshtein(qRunes, tRunes)
		maxL := max(len(qRunes), len(tRunes))
		sim := 1.0 - float64(d)/float64(maxL)
		if d < bestDist || (d == bestDist && sim > bestSim) {
			bestDist = d
			bestSim = sim
		}
	}

	// 1. Сравнение с фамилией
	if lastName != "" {
		checkTarget(queryRunes, []rune(lastName))
		if len(layoutRunes) > 0 {
			checkTarget(layoutRunes, []rune(lastName))
		}
	}
	if facultyLastName != "" && facultyLastName != lastName {
		checkTarget(queryRunes, []rune(facultyLastName))
		if len(layoutRunes) > 0 {
			checkTarget(layoutRunes, []rune(facultyLastName))
		}
	}

	// 2. Сравнение со всей строкой (ShortName)
	checkTarget(queryRunes, []rune(shortNorm))
	if len(layoutRunes) > 0 {
		checkTarget(layoutRunes, []rune(shortNorm))
	}

	// 3. Сравнение с полным ФИО
	if fullName != "" {
		checkTarget(queryRunes, []rune(fullName))
		if len(layoutRunes) > 0 {
			checkTarget(layoutRunes, []rune(fullName))
		}
	}

	// 4. Если запрос состоит из нескольких слов, проверим каждое слово запроса против фамилии
	if len(queryWords) > 1 && lastName != "" {
		for _, qw := range queryWords {
			qwRunes := []rune(qw)
			if len(qwRunes) >= 3 {
				checkTarget(qwRunes, []rune(lastName))
			}
		}
	}

	if !isTypoMatch(bestDist, bestSim, len(queryRunes)) {
		return candidateMatch{}, false
	}
	return candidateMatch{name: teacherName, dist: bestDist, similarity: bestSim}, true
}

func isTypoMatch(dist int, sim float64, qLen int) bool {
	switch {
	case dist == 0:
		return true
	case dist == 1 && qLen >= 3 && sim >= 0.60:
		return true
	case dist == 2 && qLen >= 5 && sim >= 0.65:
		return true
	case dist == 3 && qLen >= 8 && sim >= 0.60:
		return true
	default:
		return false
	}
}

func normalizeForComparison(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	return s
}

var qwertyToCyrillic = map[rune]rune{
	'q': 'й', 'w': 'ц', 'e': 'у', 'r': 'к', 't': 'е', 'y': 'н', 'u': 'г', 'i': 'ш', 'o': 'щ', 'p': 'з', '[': 'х', ']': 'ъ',
	'a': 'ф', 's': 'ы', 'd': 'в', 'f': 'а', 'g': 'п', 'h': 'р', 'j': 'о', 'k': 'л', 'l': 'д', ';': 'ж', '\'': 'э',
	'z': 'я', 'x': 'ч', 'c': 'с', 'v': 'м', 'b': 'и', 'n': 'т', 'm': 'ь', ',': 'б', '.': 'ю',
}

func convertKeyboardLayout(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if cyr, ok := qwertyToCyrillic[r]; ok {
			sb.WriteRune(cyr)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func damerauLevenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}

			del := dp[i-1][j] + 1
			ins := dp[i][j-1] + 1
			sub := dp[i-1][j-1] + cost
			dp[i][j] = min(del, ins, sub)

			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				trans := dp[i-2][j-2] + 1
				dp[i][j] = min(dp[i][j], trans)
			}
		}
	}

	return dp[la][lb]
}
