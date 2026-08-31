package domain

import "time"

type PreviewWorker struct {
	ID             string  `json:"worker_id"`
	FullName       string  `json:"full_name"`
	DocumentNumber string  `json:"document_number"`
	EmployeeCode   string  `json:"employee_code"`
	JobTitle       *string `json:"job_title,omitempty"`
	Department     *string `json:"department,omitempty"`
}
type PreviewMeal struct {
	MealType    string    `json:"meal_type"`
	DisplayName string    `json:"display_name"`
	ServiceDate time.Time `json:"service_date"`
	Start       string    `json:"start"`
	End         string    `json:"end"`
}
type ShiftPreviewRow struct {
	AssignmentID  string        `json:"assignment_id"`
	ShiftType     ShiftType     `json:"shift_type"`
	WorkDate      time.Time     `json:"-"`
	MealDate      string        `json:"meal_date"`
	Worker        PreviewWorker `json:"worker"`
	AssignedMeals []PreviewMeal `json:"assigned_meals"`
}
type PreviewRule struct{ MealType, Start, End string }
type ShiftPreviewSummary struct {
	ByMeal map[string]int `json:"by_meal"`
}
type ShiftPreview struct {
	Date       time.Time           `json:"date"`
	Summary    ShiftPreviewSummary `json:"summary"`
	Data       []ShiftPreviewRow   `json:"data"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}
