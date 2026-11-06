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

	wednesday := time.Date(2026, time.November, 4, 15, 30, 0, 0, MinskLocation)
	monday := CurrentWeekMonday(wednesday)
	if got := monday.Format(DateFormat); got != "2026-11-02" {
		t.Errorf("expected Monday 2026-11-02, got %s", got)
	}
}
