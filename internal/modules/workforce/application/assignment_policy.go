package application

import (
	"errors"
	"time"
)

var ErrAssignmentOutsideAllowedWeek = errors.New(
	"worker shift assignments can only be created, modified or deleted until 18:00 on the day before the shift (America/Lima)",
)

func CanManageAssignmentForDate(workDate time.Time, now time.Time) bool {
	workDate = normalizeDate(workDate)
	now = now.In(workDate.Location())
	deadline := workDate.AddDate(0, 0, -1).Add(18 * time.Hour)
	return !now.After(deadline)
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
