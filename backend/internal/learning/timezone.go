package learning

import (
	"regexp"
	"time"

	"learning-serbian-platform/backend/internal/apperr"
)

// timeZoneName is the shape of an IANA name such as Europe/Belgrade or
// America/Argentina/Buenos_Aires.
var timeZoneName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z0-9_+-]+)*$`)

// localDay is the day it is for a student, so that a day starts at their
// own midnight.
type localDay struct {
	timeZone string    // the IANA name, which PostgreSQL also takes
	start    time.Time // midnight in that time zone
}

// currentDay returns the day it is at now in timeZone, an IANA name; empty
// means UTC.
func currentDay(timeZone string, now time.Time) (localDay, error) {
	if timeZone == "" {
		timeZone = "UTC"
	}
	loc, err := time.LoadLocation(timeZone)
	if timeZone == "Local" || !timeZoneName.MatchString(timeZone) || err != nil {
		return localDay{}, apperr.Validation(map[string]string{
			"timeZone": "Use a time zone name such as Europe/Belgrade.",
		})
	}
	local := now.In(loc)
	return localDay{
		timeZone: timeZone,
		start:    time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc),
	}, nil
}

// date is the day's date as a date column holds it in Go: midnight UTC.
func (d localDay) date() time.Time {
	y, m, day := d.start.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}
