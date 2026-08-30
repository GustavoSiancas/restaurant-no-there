package domain

import (
	core "backend/internal/core/domain"
	"time"
)

type Role string

const (
	RoleAdmin  Role = "ADMIN"
	RoleOwner  Role = "OWNER"
	RoleRRHH   Role = "RRHH"
	RoleWorker Role = "WORKER"
)

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleOwner || r == RoleRRHH || r == RoleWorker
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
