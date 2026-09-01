package http

import (
	"backend/internal/modules/users/application"
	"backend/internal/modules/users/domain"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Handler struct{ service *application.Service }

func New(s *application.Service) *Handler { return &Handler{service: s} }

type managementRegisterRequest struct {
	Username  string      `json:"username"`
	Email     string      `json:"email"`
	Password  string      `json:"password"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Role      domain.Role `json:"role"`
}

func (h *Handler) BootstrapAdmin(c *gin.Context) {
	var r managementRegisterRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	u, err := h.service.BootstrapAdmin(c, application.RegisterInput{Username: r.Username, Email: r.Email, Password: r.Password, FirstName: r.FirstName, LastName: r.LastName})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *Handler) RegisterManagement(c *gin.Context) {
	var r managementRegisterRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	if r.Role != domain.RoleOwner && r.Role != domain.RoleRRHH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be OWNER or RRHH"})
		return
	}
	u, err := h.service.Register(c, application.RegisterInput{Username: r.Username, Email: r.Email, Password: r.Password, FirstName: r.FirstName, LastName: r.LastName, Role: r.Role})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *Handler) RegisterCollaborator(c *gin.Context) {
	var r managementRegisterRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	u, err := h.service.Register(c, application.RegisterInput{Username: r.Username, Email: r.Email, Password: r.Password, FirstName: r.FirstName, LastName: r.LastName, Role: domain.RoleCollaborator})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *Handler) My(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user not found"})
		return
	}
	user, err := h.service.FindMyUser(c, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) Workers(c *gin.Context) {
	users, err := h.service.ListByRoles(c, domain.RoleWorker)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list workers"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) Collaborators(c *gin.Context) {
	users, err := h.service.ListByRoles(c, domain.RoleCollaborator)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list collaborators"})
		return
	}
	c.JSON(http.StatusOK, users)
}
