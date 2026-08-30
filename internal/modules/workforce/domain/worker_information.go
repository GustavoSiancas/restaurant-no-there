package domain

import "time"

type WorkerInformation struct {
	UserID                string     `json:"user_id"`
	EmployeeCode          string     `json:"employee_code"`
	JobTitle              *string    `json:"job_title,omitempty"`
	Department            *string    `json:"department,omitempty"`
	Phone                 *string    `json:"phone,omitempty"`
	Address               *string    `json:"address,omitempty"`
	HireDate              *time.Time `json:"hire_date,omitempty"`
	EmergencyContactName  *string    `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone *string    `json:"emergency_contact_phone,omitempty"`
	Notes                 *string    `json:"notes,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}
