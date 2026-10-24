package entities

// GroupStudentCounts хранит точное количество учащихся по каждой группе колледжа
var GroupStudentCounts = map[int32]int{
	9:   24,
	79:  29,
	80:  30,
	81:  30,
	82:  30,
	83:  30,
	84:  28,
	85:  25,
	86:  24,
	87:  25,
	88:  30,
	89:  30,
	90:  27,
	91:  29,
	92:  29,
	93:  7,
	94:  8,
	95:  30,
	96:  25,
	97:  25,
	98:  30,
	99:  30,
	100: 30,
	101: 11,
	102: 25,
	103: 25,
	104: 30,
	105: 25,
	106: 29,
	107: 25,
	108: 25,
	109: 10,
	167: 8,
	168: 9,
	169: 7,
}

// CalculateSubgroupStudents возвращает количество студентов для группы и конкретной подгруппы.
// Если subgroup == 0, возвращается количество студентов во всей группе.
// По подгруппам деление идет на половину (например, группа из 30 человек делится на 15 + 15).
func CalculateSubgroupStudents(groupID int32, subgroup int) int {
	total, ok := GroupStudentCounts[groupID]
	if !ok {
		total = 25
	}
	if subgroup <= 0 {
		return total
	}
	if subgroup == 1 {
		return (total + 1) / 2
	}
	return total / 2
}
