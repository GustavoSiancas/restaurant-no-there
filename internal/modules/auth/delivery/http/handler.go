package http

import (
	"backend/internal/modules/auth/application"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
)

type Handler struct{ service *application.Service }

func New(s *application.Service) *Handler { return &Handler{service: s} }
func clientIP(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil {
		return host
	}
	return c.ClientIP()
}
func (h *Handler) LoginPassword(c *gin.Context) {
	var r struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	p, e := h.service.LoginPassword(c, r.Username, r.Password, c.Request.UserAgent(), clientIP(c))
	if e != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": e.Error()})
		return
	}
	c.JSON(200, p)
}
func (h *Handler) LoginDNI(c *gin.Context) {
	var r struct {
		DNI string `json:"dni"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	p, e := h.service.LoginDNI(c, r.DNI, c.Request.UserAgent(), clientIP(c))
	if e != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": e.Error()})
		return
	}
	c.JSON(200, p)
}
func (h *Handler) Refresh(c *gin.Context) {
	var r struct {
		RefreshToken string `json:"refresh_token"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	p, e := h.service.Refresh(c, r.RefreshToken, c.Request.UserAgent(), clientIP(c))
	if e != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": e.Error()})
		return
	}
	c.JSON(200, p)
}
