package handlers

import (
	"testing"
	"time"
)

func TestDataFromCallbackData(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "current week", data: "currentweek|99", want: "99"},
		{name: "week navigation", data: "prevweek|99:24.08.2026", want: "99:24.08.2026"},
		{name: "day navigation", data: "nextday|88:-3", want: "88:-3"},
		{name: "no separator", data: "broken", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dataFromCallbackData(tt.data); got != tt.want {
				t.Errorf("dataFromCallbackData(%q) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestParseCallbackWeekNavigation(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		groupID, date, err := parseCallbackWeekNavigation("prevweek|99:24.08.2026")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)
		if groupID != 99 || !date.Equal(want) {
			t.Errorf("got (%d, %v), want (99, %v)", groupID, date, want)
		}
	})

	errorCases := []struct {
		name string
		data string
	}{
		{name: "missing separator in payload", data: "prevweek|99"},
		{name: "bad group id", data: "prevweek|abc:24.08.2026"},
		{name: "bad date", data: "prevweek|99:notadate"},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := parseCallbackWeekNavigation(tt.data); err == nil {
				t.Errorf("parseCallbackWeekNavigation(%q) expected error", tt.data)
			}
		})
	}
}

func TestParseCallbackDayNavigation(t *testing.T) {
	validCases := []struct {
		name       string
		data       string
		wantGroup  int
		wantOffset int
	}{
		{name: "today", data: "currentday|88:0", wantGroup: 88, wantOffset: 0},
		{name: "back three days", data: "prevday|88:-3", wantGroup: 88, wantOffset: -3},
		{name: "forward", data: "nextday|100:1", wantGroup: 100, wantOffset: 1},
	}

	for _, tt := range validCases {
		t.Run(tt.name, func(t *testing.T) {
			groupID, offset, err := parseCallbackDayNavigation(tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if groupID != tt.wantGroup || offset != tt.wantOffset {
				t.Errorf("got (%d, %d), want (%d, %d)", groupID, offset, tt.wantGroup, tt.wantOffset)
			}
		})
	}

	errorCases := []struct {
		name string
		data string
	}{
		{name: "missing colon", data: "nextday|88"},
		{name: "bad group id", data: "nextday|x:1"},
		{name: "bad offset", data: "nextday|88:y"},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := parseCallbackDayNavigation(tt.data); err == nil {
				t.Errorf("parseCallbackDayNavigation(%q) expected error", tt.data)
			}
		})
	}
}
