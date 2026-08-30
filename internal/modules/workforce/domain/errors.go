package domain

import (
	core "backend/internal/core/domain"
	"fmt"
)

type AssignmentConflictError struct{ Existing WorkerShiftAssignment }

func (e *AssignmentConflictError) Error() string {
	return fmt.Sprintf("worker already has shift %s on %s", e.Existing.ShiftType, e.Existing.WorkDate.Format("2006-01-02"))
}

func (e *AssignmentConflictError) Unwrap() error { return core.ErrConflict }
