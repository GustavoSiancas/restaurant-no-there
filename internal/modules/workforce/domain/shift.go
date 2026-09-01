package domain

import "time"

type ShiftType string
type ShiftStatus string

const (
	ShiftDay   ShiftType = "DAY"
	ShiftNight ShiftType = "NIGHT"
)

const (
	ShiftOpen   ShiftStatus = "OPEN"
	ShiftClosed ShiftStatus = "CLOSED"
)

type WorkerShiftAssignment struct {
	ID         string      `json:"id"`
	WorkerID   string      `json:"worker_id"`
	ShiftType  ShiftType   `json:"shift_type"`
	Status     ShiftStatus `json:"status"`
	WorkDate   time.Time   `json:"work_date"`
	AssignedBy string      `json:"assigned_by"`
	Notes      *string     `json:"notes,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}
