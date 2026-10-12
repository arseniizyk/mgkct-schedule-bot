package notifier

import (
	"context"
	"testing"
	"time"
)

func TestCalculateTomorrowDate(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)

	tests := []struct {
		name       string
		now        time.Time
		wantDate   string
		wantDayIdx int
	}{
		{
			name:       "Monday -> Tuesday (idx 1)",
			now:        time.Date(2026, 10, 12, 20, 0, 0, 0, minskLoc),
			wantDate:   "13.10.2026",
			wantDayIdx: 1,
		},
		{
			name:       "Tuesday -> Wednesday (idx 2)",
			now:        time.Date(2026, 10, 13, 20, 0, 0, 0, minskLoc),
			wantDate:   "14.10.2026",
			wantDayIdx: 2,
		},
		{
			name:       "Wednesday -> Thursday (idx 3)",
			now:        time.Date(2026, 10, 14, 20, 0, 0, 0, minskLoc),
			wantDate:   "15.10.2026",
			wantDayIdx: 3,
		},
		{
			name:       "Thursday -> Friday (idx 4)",
			now:        time.Date(2026, 10, 15, 20, 0, 0, 0, minskLoc),
			wantDate:   "16.10.2026",
			wantDayIdx: 4,
		},
		{
			name:       "Friday -> Saturday (idx 5)",
			now:        time.Date(2026, 10, 16, 20, 0, 0, 0, minskLoc),
			wantDate:   "17.10.2026",
			wantDayIdx: 5,
		},
		{
			name:       "Saturday -> Monday (idx 0)",
			now:        time.Date(2026, 10, 17, 20, 0, 0, 0, minskLoc),
			wantDate:   "19.10.2026",
			wantDayIdx: 0,
		},
		{
			name:       "Sunday -> Monday (idx 0)",
			now:        time.Date(2026, 10, 18, 20, 0, 0, 0, minskLoc),
			wantDate:   "19.10.2026",
			wantDayIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDate, gotDayIdx := CalculateTomorrowDate(tt.now)
			if gotDate.Format("02.01.2006") != tt.wantDate {
				t.Errorf("gotDate = %s, want %s", gotDate.Format("02.01.2006"), tt.wantDate)
			}
			if gotDayIdx != tt.wantDayIdx {
				t.Errorf("gotDayIdx = %d, want %d", gotDayIdx, tt.wantDayIdx)
			}
		})
	}
}

func TestNotifier_ScraperAlerts_NoAdmin(t *testing.T) {
	n := &Notifier{
		adminChatID: 0,
	}

	ctx := context.Background()
	if err := n.HandleScraperError(ctx, nil, 5); err != nil {
		t.Errorf("expected nil error when adminChatID=0, got %v", err)
	}
	if err := n.HandleScraperRecovered(ctx); err != nil {
		t.Errorf("expected nil error when adminChatID=0, got %v", err)
	}
}
