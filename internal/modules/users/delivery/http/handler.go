package http

import (
	"backend/internal/modules/users/application"
	"backend/internal/modules/users/domain"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Handler struct{ service *application.Service }

func New(s *application.Service) *Handler { return &Handler{service: s} }

type registerRequest struct {
	Username  string      `json:"username"`
	DNI       string      `json:"dni"`
	Email     string      `json:"email"`
	Password  string      `json:"password"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Role      domain.Role `json:"role"`
}

func (h *Handler) Register(c *gin.Context) {
	var r registerRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	u, err := h.service.Register(c, application.RegisterInput{Username: r.Username, DNI: r.DNI, Email: r.Email, Password: r.Password, FirstName: r.FirstName, LastName: r.LastName, Role: r.Role})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}
func (h *Handler) List(c *gin.Context) {
	users, err := h.service.List(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "could not list users"})
		return
	}
	c.JSON(200, users)
}
func (h *Handler) Get(c *gin.Context) {
	u, err := h.service.FindByID(c, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.JSON(200, u)
}
