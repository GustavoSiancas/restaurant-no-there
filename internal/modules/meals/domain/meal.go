package domain

import "time"

type MealType string

const (
	Breakfast MealType = "DESAYUNO"
	Afternoon MealType = "TARDE"
	Night     MealType = "NOCHE"
)

func (m MealType) Valid() bool { return m == Breakfast || m == Afternoon || m == Night }

type ServiceRule struct {
	MealType    MealType `json:"meal_type"`
	ClaimStart  string   `json:"claim_start"`
	ClaimEnd    string   `json:"claim_end"`
	Timezone    string   `json:"timezone"`
	Description string   `json:"description"`
	Active      bool     `json:"active"`
}

type Claim struct {
	ID                string    `json:"id"`
	WorkerID          string    `json:"worker_id"`
	ShiftAssignmentID string    `json:"shift_assignment_id"`
	MealType          MealType  `json:"meal_type"`
	ServiceDate       time.Time `json:"service_date"`
	ClaimedAt         time.Time `json:"claimed_at"`
	Notes             *string   `json:"notes,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ReportRow struct {
	MealType   MealType `json:"meal_type"`
	Eligible   int64    `json:"eligible"`
	Claimed    int64    `json:"claimed"`
	NotClaimed int64    `json:"not_claimed"`
}

type CurrentShift struct {
	AssignmentID string    `json:"assignment_id"`
	ShiftType    string    `json:"shift_type"`
	WorkDate     time.Time `json:"work_date"`
}

type CurrentMeal struct {
	MealType       MealType `json:"meal_type"`
	WindowStart    string   `json:"window_start"`
	WindowEnd      string   `json:"window_end"`
	Eligible       bool     `json:"eligible"`
	CanClaim       bool     `json:"can_claim"`
	AlreadyClaimed bool     `json:"already_claimed"`
	ClaimID        *string  `json:"claim_id,omitempty"`
}

type WorkerStatus struct {
	PeruTime       time.Time     `json:"peru_time"`
	OnShift        bool          `json:"on_shift"`
	CurrentShift   *CurrentShift `json:"current_shift,omitempty"`
	MealWindowOpen bool          `json:"meal_window_open"`
	CurrentMeal    *CurrentMeal  `json:"current_meal,omitempty"`
}

type WorkerTicketIdentity struct {
	ID        string
	FirstName string
	LastName  string
	DNI       string
}

type ClaimPreviewWorker struct {
	ID             string `json:"id"`
	FullName       string `json:"fullName"`
	DocumentNumber string `json:"documentNumber"`
}

type ClaimPreviewService struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type ClaimPreview struct {
	RedemptionID string               `json:"redemptionId,omitempty"`
	TicketNumber string               `json:"ticketNumber,omitempty"`
	Status       string               `json:"status"`
	Worker       ClaimPreviewWorker   `json:"worker"`
	Service      *ClaimPreviewService `json:"service,omitempty"`
	Date         string               `json:"date"`
	Time         string               `json:"time"`
	Reason       string               `json:"reason,omitempty"`
}
