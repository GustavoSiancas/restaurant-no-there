package domain

import (
	"time"

	core "backend/internal/core/domain"
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
	Username     *string    `json:"username,omitempty"`
	DNI          *string    `json:"dni,omitempty"`
	Email        *string    `json:"email,omitempty"`
	PasswordHash *string    `json:"-"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	Role         Role       `json:"role"`
	Active       bool       `json:"active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}
