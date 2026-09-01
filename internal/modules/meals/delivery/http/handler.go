package http

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "backend/internal/core/domain"
	"backend/internal/modules/meals/application"
	"backend/internal/modules/meals/domain"
	reportexcel "backend/internal/modules/meals/infrastructure/excel"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *application.Service
	broker  *Broker
}

func New(service *application.Service, broker *Broker) *Handler {
	return &Handler{service: service, broker: broker}
}

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
	if order, findErr := h.service.FindOrder(c, claim.ID); findErr == nil {
		h.broker.Publish(OrderEvent{Type: "MEAL_ORDER_CREATED", Data: order})
	}
	c.JSON(http.StatusCreated, gin.H{"claim_created": true, "claim": claim})
}

func (h *Handler) ListOrders(c *gin.Context) {
	orders, err := h.service.ListOrders(c, domain.ClaimStatus(c.DefaultQuery("status", string(domain.ClaimClaimed))))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

func (h *Handler) GetOrder(c *gin.Context) {
	order, err := h.service.FindOrder(c, c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "meal order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *Handler) ValidateOrder(c *gin.Context) {
	collaboratorID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user not found"})
		return
	}
	order, err := h.service.ValidateOrder(c, c.Param("id"), collaboratorID.(string))
	if err != nil {
		if errors.Is(err, core.ErrConflict) {
			code, message := "MEAL_ORDER_ALREADY_VALIDATED", "el pedido ya fue entregado"
			if order != nil && order.Status == domain.ClaimClaimed {
				code, message = "MEAL_ORDER_WINDOW_CLOSED", "el horario para validar el pedido ya terminó"
			} else if order != nil && order.Status == domain.ClaimNotClaimed {
				code, message = "MEAL_ORDER_NOT_CLAIMED", "el trabajador no recogió esta comida dentro del horario"
			} else if order != nil && order.Status == domain.ClaimClaimedNotValidated {
				code, message = "MEAL_ORDER_CLAIMED_NOT_VALIDATED", "el pedido no fue validado dentro del horario"
			}
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": code, "message": message, "order": order}})
			return
		}
		if errors.Is(err, core.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "meal order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not validate meal order"})
		return
	}
	h.broker.Publish(OrderEvent{Type: "MEAL_ORDER_VALIDATED", Data: order})
	c.JSON(http.StatusOK, order)
}

func (h *Handler) OrdersWebSocket(c *gin.Context) { h.broker.Serve(c) }

func detailedReportQuery(c *gin.Context) (domain.ReportFilters, int, int, error) {
	var filters domain.ReportFilters
	var err error
	if value := strings.TrimSpace(c.Query("from")); value != "" {
		filters.From, err = time.Parse("2006-01-02", value)
		if err != nil {
			return filters, 0, 0, fmt.Errorf("from must use YYYY-MM-DD")
		}
	}
	if value := strings.TrimSpace(c.Query("to")); value != "" {
		filters.To, err = time.Parse("2006-01-02", value)
		if err != nil {
			return filters, 0, 0, fmt.Errorf("to must use YYYY-MM-DD")
		}
	}
	filters.MealType = domain.MealType(strings.ToUpper(strings.TrimSpace(c.Query("meal_type"))))
	filters.ShiftType = strings.ToUpper(strings.TrimSpace(c.Query("shift_type")))
	page, pageSize := 1, 20
	if value := c.Query("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return filters, 0, 0, fmt.Errorf("page must be a positive integer")
		}
	}
	if value := c.Query("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 {
			return filters, 0, 0, fmt.Errorf("page_size must be a positive integer")
		}
	}
	return filters, page, pageSize, nil
}

func (h *Handler) DetailedReport(c *gin.Context) {
	filters, page, pageSize, err := detailedReportQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report, err := h.service.DetailedReport(c, filters, page, pageSize, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handler) ExportDetailedReport(c *gin.Context) {
	filters, _, _, err := detailedReportQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report, err := h.service.DetailedReport(c, filters, 1, 20, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	content, err := reportexcel.BuildDetailedReport(report)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate Excel report"})
		return
	}
	filename := fmt.Sprintf("reporte-comidas-%s-%s.xlsx", report.Filters.From.Format("20060102"), report.Filters.To.Format("20060102"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
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
