package application

import (
	"errors"
	"time"
)

var ErrAssignmentOutsideAllowedWeek = errors.New(
	"worker shift assignments can only be created or modified for next week",
)

func CanManageAssignmentForDate(workDate time.Time, now time.Time) bool {
	workDate = normalizeDate(workDate)
	now = normalizeDate(now)

	currentWeekStart := startOfWeek(now)

	nextWeekStart := currentWeekStart.AddDate(0, 0, 7)
	nextWeekEnd := nextWeekStart.AddDate(0, 0, 6)

	return !workDate.Before(nextWeekStart) &&
		!workDate.After(nextWeekEnd)
}

func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())

	// Go considera Sunday = 0
	if weekday == 0 {
		weekday = 7
	}

	monday := t.AddDate(0, 0, -(weekday - 1))

	return normalizeDate(monday)
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