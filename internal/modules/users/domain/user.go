package domain

import (
	core "backend/internal/core/domain"
	"time"
)

type Role string

const (
	RoleAdmin        Role = "ADMIN"
	RoleOwner        Role = "OWNER"
	RoleRRHH         Role = "RRHH"
	RoleWorker       Role = "WORKER"
	RoleCollaborator Role = "COLLABORATOR"
)

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleOwner || r == RoleRRHH || r == RoleWorker || r == RoleCollaborator
}

type User struct {
	core.Entity
	Role        Role       `json:"role"`
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type Profile struct {
	UserID    string  `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
}

type CredentialType string

const (
	CredentialPassword CredentialType = "PASSWORD"
	CredentialDNI      CredentialType = "DNI"
	CredentialFaceScan CredentialType = "FACE_SCAN"
)

type PublicCredential struct {
	Type       CredentialType `json:"type"`
	Identifier string         `json:"identifier"`
}

type WorkerDetails struct {
	EmployeeCode          string     `json:"employee_code"`
	PhotoURL              *string    `json:"photo_url,omitempty"`
	JobTitle              *string    `json:"job_title,omitempty"`
	Department            *string    `json:"department,omitempty"`
	Phone                 *string    `json:"phone,omitempty"`
	Address               *string    `json:"address,omitempty"`
	HireDate              *time.Time `json:"hire_date,omitempty"`
	EmergencyContactName  *string    `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone *string    `json:"emergency_contact_phone,omitempty"`
	Notes                 *string    `json:"notes,omitempty"`
}

type MyUser struct {
	User
	Profile     Profile            `json:"profile"`
	Credentials []PublicCredential `json:"credentials"`
	Worker      *WorkerDetails     `json:"worker_information,omitempty"`
}
