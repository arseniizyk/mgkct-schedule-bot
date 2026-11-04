package timeutil

import (
	"testing"
	"time"
)

func TestTimeutil(t *testing.T) {
	now := Now()
	if now.Location().String() != "Europe/Minsk" {
		t.Errorf("expected Europe/Minsk, got %s", now.Location().String())
	}

	today := TodayStr()
	if len(today) != 10 {
		t.Errorf("expected 10 chars date string, got %s", today)
	}

	fixed := time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)
	formatted := FormatDate(fixed)
	if formatted != "2025-09-01" {
		t.Errorf("expected 2025-09-01, got %s", formatted)
	}
}
