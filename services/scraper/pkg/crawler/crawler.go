package crawler

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

const url = `https://mgkct.minskedu.gov.by/%D0%BF%D0%B5%D1%80%D1%81%D0%BE%D0%BD%D0%B0%D0%BB%D0%B8%D0%B8/%D1%83%D1%87%D0%B0%D1%89%D0%B8%D0%BC%D1%81%D1%8F/%D1%80%D0%B0%D1%81%D0%BF%D0%B8%D1%81%D0%B0%D0%BD%D0%B8%D0%B5-%D0%B7%D0%B0%D0%BD%D1%8F%D1%82%D0%B8%D0%B9-%D0%BD%D0%B0-%D0%BD%D0%B5%D0%B4%D0%B5%D0%BB%D1%8E`

type Crawler struct {
	c *colly.Collector
}

func New() *Crawler {
	return &Crawler{
		c: colly.NewCollector(),
	}
}

func (c *Crawler) Crawl() {
	test := make(map[int]int)

	c.c.OnHTML("h2", func(e *colly.HTMLElement) {
		r := regexp.MustCompile(`\d+`)
		matched := r.FindString(e.Text)
		num, err := strconv.Atoi(matched)
		if err != nil {
			if strings.Contains(e.Text, "Кол") || strings.Contains(e.Text, "Техн") {
				return
			}

			slog.Error("can't parse text to int", "text", e.Text, "err", err)
			return
		}
		test[num]++
	})

	c.c.Visit(url)

	fmt.Println(test)
}
