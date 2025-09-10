package parser

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

var ErrBadGroup = errors.New("кол группа")

func parseGroup(text string) (int, error) {
	if strings.Contains(text, "Кол") || strings.Contains(text, "кол") {
		return 0, ErrBadGroup
	}

	r := regexp.MustCompile(`\d+`)
	matched := r.FindString(text)
	if matched == "" {
		return 0, fmt.Errorf("no digits in %q", text)
	}

	group, err := strconv.Atoi(matched)
	if err != nil {
		slog.Error("can't parse group to int", "text", text, "err", err)
		return 0, err
	}

	return group, nil
}

func parseWeek(e *colly.HTMLElement) time.Time {
	layout := "02.01.2006"

	parts := strings.Split(e.Text, " - ")
	start, _ := time.Parse(layout, parts[0])
	return start
}
