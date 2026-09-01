package application

import (
	"errors"
	"time"
)

var ErrAssignmentOutsideAllowedWeek = errors.New(
	"worker shift assignments can only be created or modified until the day before the shift",
)

func CanManageAssignmentForDate(workDate time.Time, now time.Time) bool {
	workDate = normalizeDate(workDate)
	now = normalizeDate(now)

	return workDate.After(now)
}

func normalizeDate(t time.Time) time.Time {
	return time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		0, 0, 0, 0,
		t.Location(),
	)
}
