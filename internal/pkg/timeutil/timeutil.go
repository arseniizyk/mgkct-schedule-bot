package timeutil

import "time"

// MinskLocation holds the standard Minsk (UTC+3) timezone location.
var MinskLocation = time.FixedZone("Europe/Minsk", 3*3600)

// DateFormat is the standard ISO date format (YYYY-MM-DD).
const DateFormat = "2006-01-02"

// Now returns the current time in Minsk timezone.
func Now() time.Time {
	return time.Now().In(MinskLocation)
}

// InMinsk converts a given time to Minsk timezone.
func InMinsk(t time.Time) time.Time {
	return t.In(MinskLocation)
}

// TodayStr returns the current date string in YYYY-MM-DD format in Minsk timezone.
func TodayStr() string {
	return Now().Format(DateFormat)
}

// CurrentWeekMonday returns midnight Monday (Europe/Minsk, UTC+3) for the week containing now.
func CurrentWeekMonday(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	localNow := now.In(MinskLocation)
	offset := (int(localNow.Weekday()) + 6) % 7 // Mon=0, Tue=1, ..., Sun=6
	return time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, MinskLocation).AddDate(0, 0, -offset)
}
