package http

import (
	"errors"
	"net/http"
	"time"

	core "backend/internal/core/domain"
	"backend/internal/modules/meals/application"
	"backend/internal/modules/meals/domain"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *application.Service }

func New(service *application.Service) *Handler { return &Handler{service: service} }

type confirmPrintRequest struct {
	MealType domain.MealType `json:"meal_type"`
	Printed  bool            `json:"printed"`
	Notes    string          `json:"notes"`
}

func (h *Handler) ClaimPreview(c *gin.Context) {
	workerID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user not found"})
		return
	}
	preview, err := h.service.ClaimPreview(c, workerID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not obtain meal claim preview"})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (h *Handler) ConfirmPrint(c *gin.Context) {
	var r confirmPrintRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	if !r.Printed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "printed must be true; no meal claim was created"})
		return
	}
	workerID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user not found"})
		return
	}
	claim, err := h.service.Claim(c, workerID.(string), r.MealType, r.Notes)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, core.ErrConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error(), "claim_created": false})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"claim_created": true, "claim": claim})
}
func (h *Handler) Report(c *gin.Context) {
	from, e1 := time.Parse("2006-01-02", c.Query("from"))
	to, e2 := time.Parse("2006-01-02", c.Query("to"))
	if e1 != nil || e2 != nil {
		c.JSON(400, gin.H{"error": "from and to must use YYYY-MM-DD"})
		return
	}
	report, err := h.service.Report(c, from, to)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, report)
}

func (h *Handler) ListSchedules(c *gin.Context) {
	schedules, err := h.service.ListSchedules(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not obtain meal schedules"})
		return
	}
	c.JSON(http.StatusOK, schedules)
}

func (h *Handler) WorkerStatus(c *gin.Context) {
	workerID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user not found"})
		return
	}
	status, err := h.service.WorkerStatus(c, workerID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not obtain worker status"})
		return
	}
	c.JSON(http.StatusOK, status)
}
