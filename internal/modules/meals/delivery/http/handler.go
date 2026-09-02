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
	h.publishClaimedOrders(c)
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
	h.publishClaimedOrders(c)
	c.JSON(http.StatusOK, order)
}

func (h *Handler) publishClaimedOrders(c *gin.Context) {
	orders, err := h.service.ListOrders(c, domain.ClaimClaimed)
	if err == nil {
		h.broker.Publish(OrderEvent{Type: "CLAIMED_ORDERS", Data: orders})
	}
}

func (h *Handler) OrdersWebSocket(c *gin.Context) {
	orders, err := h.service.ListOrders(c, domain.ClaimClaimed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not obtain claimed meal orders"})
		return
	}
	h.broker.Serve(c, orders)
}

func (h *Handler) DailyMealStatusReport(c *gin.Context) {
	date, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("date")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date is required and must use YYYY-MM-DD"})
		return
	}
	mealTypes := make([]domain.MealType, 0)
	for _, value := range c.QueryArray("meal_type") {
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				mealTypes = append(mealTypes, domain.MealType(strings.ToUpper(item)))
			}
		}
	}
	page, pageSize := 1, 20
	if value := c.Query("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a positive integer"})
			return
		}
	}
	if value := c.Query("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page_size must be between 1 and 100"})
			return
		}
	}
	report, err := h.service.DailyMealStatusReport(c, date, mealTypes, page, pageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func mealStatusReportQuery(c *gin.Context) (time.Time, time.Time, int, int, error) {
	from, fromErr := time.Parse("2006-01-02", strings.TrimSpace(c.Query("from")))
	to, toErr := time.Parse("2006-01-02", strings.TrimSpace(c.Query("to")))
	if fromErr != nil || toErr != nil {
		return time.Time{}, time.Time{}, 0, 0, fmt.Errorf("from and to are required and must use YYYY-MM-DD")
	}
	page, pageSize := 1, 20
	var err error
	if value := c.Query("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return time.Time{}, time.Time{}, 0, 0, fmt.Errorf("page must be a positive integer")
		}
	}
	if value := c.Query("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > 100 {
			return time.Time{}, time.Time{}, 0, 0, fmt.Errorf("page_size must be between 1 and 100")
		}
	}
	return from, to, page, pageSize, nil
}

func (h *Handler) MealStatusReport(c *gin.Context) {
	from, to, page, pageSize, err := mealStatusReportQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report, err := h.service.MealStatusReport(c, from, to, page, pageSize, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handler) ExportMealStatusReport(c *gin.Context) {
	from, to, _, _, err := mealStatusReportQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report, err := h.service.MealStatusReport(c, from, to, 1, 100, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	content, err := reportexcel.BuildMealStatusReport(report)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate Excel report"})
		return
	}
	filename := fmt.Sprintf("estados-comidas-%s-%s.xlsx", report.From.Format("20060102"), report.To.Format("20060102"))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
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
