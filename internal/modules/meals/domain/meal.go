package domain

import "time"

type MealType string
type ClaimStatus string

const (
	Breakfast MealType = "BREAKFAST"
	Lunch     MealType = "LUNCH"
	Dinner    MealType = "DINNER"
)

const (
	ClaimCreated             ClaimStatus = "CREATED"
	ClaimClaimed             ClaimStatus = "CLAIMED"
	ClaimNotClaimed          ClaimStatus = "NOT_CLAIMED"
	ClaimClaimedNotValidated ClaimStatus = "CLAIMED_BUT_NOT_VALIDATED"
	ClaimValidated           ClaimStatus = "VALIDATED"
)

func (m MealType) Valid() bool { return m == Breakfast || m == Lunch || m == Dinner }

type ServiceRule struct {
	MealType    MealType `json:"meal_type"`
	ClaimStart  string   `json:"claim_start"`
	ClaimEnd    string   `json:"claim_end"`
	Timezone    string   `json:"timezone"`
	Description string   `json:"description"`
	Active      bool     `json:"active"`
}

type Claim struct {
	ID                string      `json:"id"`
	WorkerID          string      `json:"worker_id"`
	ShiftAssignmentID string      `json:"shift_assignment_id"`
	MealType          MealType    `json:"meal_type"`
	ServiceDate       time.Time   `json:"service_date"`
	ClaimedAt         *time.Time  `json:"claimed_at,omitempty"`
	Status            ClaimStatus `json:"status"`
	ValidatedAt       *time.Time  `json:"validated_at,omitempty"`
	ValidatedBy       *string     `json:"validated_by,omitempty"`
	Notes             *string     `json:"notes,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type MealOrder struct {
	Claim
	Worker  ClaimPreviewWorker  `json:"worker"`
	Service ClaimPreviewService `json:"service"`
}

type ReportRow struct {
	MealType   MealType `json:"meal_type"`
	Eligible   int64    `json:"eligible"`
	Claimed    int64    `json:"claimed"`
	NotClaimed int64    `json:"not_claimed"`
}

type ReportFilters struct {
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	MealType  MealType  `json:"meal_type,omitempty"`
	ShiftType string    `json:"shift_type,omitempty"`
}

type DetailedReportRow struct {
	ID             string      `json:"id"`
	ServiceDate    time.Time   `json:"service_date"`
	MealType       MealType    `json:"meal_type"`
	ShiftType      string      `json:"shift_type"`
	Status         ClaimStatus `json:"status"`
	ClaimedAt      *time.Time  `json:"claimed_at,omitempty"`
	ValidatedAt    *time.Time  `json:"validated_at,omitempty"`
	WorkerID       string      `json:"worker_id"`
	FullName       string      `json:"full_name"`
	DocumentNumber string      `json:"document_number"`
	EmployeeCode   string      `json:"employee_code"`
	Department     *string     `json:"department,omitempty"`
}

type DetailedReportSummary struct {
	TotalEligible         int64 `json:"total_eligible"`
	Consumed              int64 `json:"consumed"`
	RequestedNotValidated int64 `json:"requested_not_validated"`
	NotClaimed            int64 `json:"not_claimed"`
	DidNotConsume         int64 `json:"did_not_consume"`
}

type DetailedReport struct {
	Filters    ReportFilters         `json:"filters"`
	Summary    DetailedReportSummary `json:"summary"`
	Data       []DetailedReportRow   `json:"data"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	Total      int64                 `json:"total"`
	TotalPages int                   `json:"total_pages"`
}

type MealWindowClosure struct {
	NotConsumed           int64 `json:"not_consumed"`
	RequestedNotValidated int64 `json:"requested_but_not_validated"`
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

type AssignedMeal struct {
	MealType    MealType `json:"meal_type"`
	DisplayName string   `json:"display_name"`
	ServiceDate string   `json:"service_date"`
}

type WorkerStatus struct {
	PeruTime       time.Time      `json:"peru_time"`
	OnShift        bool           `json:"on_shift"`
	CurrentShift   *CurrentShift  `json:"current_shift,omitempty"`
	AssignedMeals  []AssignedMeal `json:"assigned_meals"`
	MealWindowOpen bool           `json:"meal_window_open"`
	CurrentMeal    *CurrentMeal   `json:"current_meal,omitempty"`
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
