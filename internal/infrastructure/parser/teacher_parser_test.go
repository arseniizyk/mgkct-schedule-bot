package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitTeacherCellSubject_UnicodeDashes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantGroup   string
		wantSubject string
	}{
		{
			name:        "standard hyphen",
			input:       "101 - Программирование",
			wantGroup:   "101",
			wantSubject: "Программирование",
		},
		{
			name:        "unicode en-dash",
			input:       "201 – Основы алгоритмизации",
			wantGroup:   "201",
			wantSubject: "Основы алгоритмизации",
		},
		{
			name:        "unicode em-dash",
			input:       "301 — Базы данных",
			wantGroup:   "301",
			wantSubject: "Базы данных",
		},
		{
			name:        "compact hyphen without spaces",
			input:       "107-Бел Яз",
			wantGroup:   "107",
			wantSubject: "Бел Яз",
		},
		{
			name:        "compact hyphen with star group",
			input:       "169*-КПиЯП",
			wantGroup:   "169*",
			wantSubject: "КПиЯП",
		},
		{
			name:        "no separator",
			input:       "Дежурство",
			wantGroup:   "",
			wantSubject: "Дежурство",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotGroup, gotSubj := splitTeacherCellSubject(tt.input)
			assert.Equal(t, tt.wantGroup, gotGroup)
			assert.Equal(t, tt.wantSubject, gotSubj)
			if gotSubj != "" {
				assert.NotEmpty(t, gotSubj)
				assert.NotEmpty(t, string([]rune(gotSubj)[0]))
			}
		})
	}
}

func TestParseTeacherName_SurnamesWithKol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    string
		wantErr error
	}{
		{input: "Преподаватель - Соколов Н. Д.", want: "Соколов Н. Д."},
		{input: "Преподаватель – Николаева Л. Н.", want: "Николаева Л. Н."},
		{input: "Преподаватель — Колесников А. В.", want: "Колесников А. В."},
		{input: "Колледж МГКЦТ", wantErr: ErrBadTeacher},
	}

	for _, tt := range tests {
		got, err := parseTeacherName(tt.input)
		if tt.wantErr != nil {
			require.ErrorIs(t, err, tt.wantErr)
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
}

func TestTeacherRe_Dashes(t *testing.T) {
	t.Parallel()

	variants := []string{
		"Преподаватель - Иванов И. И.",
		"Преподаватель – Петров П. П.",
		"Преподаватель — Сидоров С. С.",
	}

	for _, v := range variants {
		match := teacherRe.FindStringSubmatch(v)
		require.Len(t, match, 2, "must match string %q", v)
		assert.NotEmpty(t, match[1])
	}
}
