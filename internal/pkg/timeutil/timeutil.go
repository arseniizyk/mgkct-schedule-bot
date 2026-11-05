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

// FormatDate formats a given time in YYYY-MM-DD format in Minsk timezone.
func FormatDate(t time.Time) string {
	return InMinsk(t).Format(DateFormat)
}
