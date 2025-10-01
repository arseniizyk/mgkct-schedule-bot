package parser

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/utils"
)

func parseRows(trs *goquery.Selection) []*pb.Day {
	res := make([]*pb.Day, 6)
	for i, d := range days {
		res[i] = &pb.Day{Name: d}
	}

	for row := 2; row < trs.Length(); row++ {
		tds := trs.Eq(row).Find("td")
		parseColumns(tds, res)
	}

	return res
}

func parseColumns(tds *goquery.Selection, days []*pb.Day) {
	for col := 0; col < tds.Length(); col += 2 {
		daysIdx := col / 2

		nameParts := splitByBr(tds.Eq(col))
		classParts := splitByBr(tds.Eq(col + 1))

		if len(nameParts) == 0 {
			days[daysIdx].Subjects = append(days[daysIdx].Subjects, &pb.Subject{IsEmpty: true})
			continue
		}

		pairs := parsePairs(nameParts, classParts)
		days[daysIdx].Subjects = append(days[daysIdx].Subjects, &pb.Subject{
			Pairs:   pairs,
			IsEmpty: false,
		})
	}
}

func parsePairs(nameParts, classParts []string) []*pb.Pair {
	var pairs []*pb.Pair

	for i := 0; i < len(nameParts); {
		var subjectType, teacher, class string
		name := nameParts[i]
		i++

		if len(name) > 3 && name[1] == '.' {
			name = name[:2] + " " + name[2:]
		}

		if i < len(nameParts) && strings.HasPrefix(nameParts[i], "(") {
			subjectType = nameParts[i]
			subjectType, _ = strings.CutPrefix(subjectType, "(")
			subjectType, _ = strings.CutSuffix(subjectType, ")")
			i++
		}

		if i < len(nameParts) {
			teacher = nameParts[i]
			i++
		}

		class = classParts[len(pairs)]
		class = strings.ReplaceAll(class, "(к)", "")

		pairs = append(pairs, &pb.Pair{
			Name:    utils.CleanText(name),
			Type:    utils.CleanText(subjectType),
			Teacher: utils.CleanText(teacher),
			Class:   utils.CleanText(class),
		})
	}

	return pairs
}

func splitByBr(td *goquery.Selection) []string {
	html, err := td.Html()
	if err != nil {
		slog.Warn("splitByBr: can't get html content", "err", err)
		return nil
	}

	re := regexp.MustCompile(`(?i)<br\s*/?>`)
	parts := re.Split(html, -1)

	res := make([]string, 0, len(parts))
	for _, p := range parts {
		text := utils.CleanText(p)
		if text != "" {
			res = append(res, text)
		}
	}
	return res
}
