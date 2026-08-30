package domain

import "time"

type ShiftType string

const (
	ShiftDay   ShiftType = "DIA"
	ShiftNight ShiftType = "NOCHE"
)

type Shift struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        ShiftType `json:"type"`
	Description string    `json:"description"`
	StartTime   string    `json:"start_time"`
	EndTime     string    `json:"end_time"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkerShiftAssignment struct {
	ID         string    `json:"id"`
	WorkerID   string    `json:"worker_id"`
	ShiftID    string    `json:"shift_id"`
	WorkDate   time.Time `json:"work_date"`
	AssignedBy string    `json:"assigned_by"`
	Notes      *string   `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
