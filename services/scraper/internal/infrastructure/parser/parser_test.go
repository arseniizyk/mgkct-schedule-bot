package parser

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func newSelectionText(text string) *goquery.Selection {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<p>" + text + "</p>"))
	if err != nil {
		panic(err)
	}
	return doc.Find("p").First()
}

func TestParseGroup(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    int32
		wantErr error
	}{
		{name: "simple group", text: "Группа 99", want: 99},
		{name: "group with trailing text", text: "Группа 123 (П)", want: 123},
		{name: "only digits", text: "42", want: 42},
		{name: "bad group Кол", text: "Колледж 5", wantErr: ErrBadGroup},
		{name: "bad group кол lowercase", text: "кол хор", wantErr: ErrBadGroup},
		{name: "no digits", text: "Группа без цифр", wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGroup(tt.text)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseGroup(%q) error = %v, want %v", tt.text, err, tt.wantErr)
				}
				return
			}

			if tt.text == "Группа без цифр" {
				if err == nil {
					t.Fatalf("parseGroup(%q) expected error for no digits", tt.text)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseGroup(%q) unexpected error: %v", tt.text, err)
			}
			if got != tt.want {
				t.Errorf("parseGroup(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseWeek(t *testing.T) {
	t.Run("valid week header", func(t *testing.T) {
		sel := newSelectionText("24.08.2026 - 30.08.2026")

		got, err := parseWeek(sel)
		if err != nil {
			t.Fatalf("parseWeek() unexpected error: %v", err)
		}

		want := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("parseWeek() = %v, want %v", got, want)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		sel := newSelectionText("не дата - 30.08.2026")

		if _, err := parseWeek(sel); err == nil {
			t.Error("parseWeek() expected error for invalid date")
		}
	})
}
