package app

import (
	"testing"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestExtractUniqueRecipients(t *testing.T) {
	users := []entities.User{
		{ChatID: 1, Group: 88, Subgroup: 1},
		{ChatID: 2, Group: 88, Subgroup: 1}, // duplicate target
		{ChatID: 3, Group: 88, Subgroup: 2},
		{ChatID: 4, Group: 88, Subgroup: 0},
		{ChatID: 5, TeacherName: "Иванов И. И."},
		{ChatID: 6, TeacherName: "Иванов И. И."}, // duplicate teacher
		{ChatID: 7, TeacherName: "Петров П. П."},
		{ChatID: 8}, // no group or teacher
	}

	targets, teachers := extractUniqueRecipients(users)

	assert.Len(t, targets, 3)
	assert.Contains(t, targets, groupSubgroupKey{groupID: 88, subgroup: 1})
	assert.Contains(t, targets, groupSubgroupKey{groupID: 88, subgroup: 2})
	assert.Contains(t, targets, groupSubgroupKey{groupID: 88, subgroup: 0})

	assert.Len(t, teachers, 2)
	assert.Contains(t, teachers, "Иванов И. И.")
	assert.Contains(t, teachers, "Петров П. П.")
}
