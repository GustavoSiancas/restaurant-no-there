package http

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/foods/application"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

type FoodDayHandler struct{ service *application.FoodDayService }

func NewFoodDayHandler(service *application.FoodDayService) *FoodDayHandler {
	return &FoodDayHandler{service: service}
}

func (h *FoodDayHandler) Create(c *gin.Context) {
	var in application.CreateFoodDayInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	day, err := h.service.Create(c, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": day.ID, "service_date": day.ServiceDate.Format("2006-01-02"), "meal_type": day.MealType, "food_id": day.FoodID, "status": day.Status, "created_at": day.CreatedAt, "updated_at": day.UpdatedAt})
}
func (h *FoodDayHandler) List(c *gin.Context) {
	groups, err := h.service.List(c, c.Query("date"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, groups)
}
func (h *FoodDayHandler) Delete(c *gin.Context) {
	err := h.service.Delete(c, c.Param("id"))
	if errors.Is(err, core.ErrLocked) {
		c.JSON(http.StatusConflict, gin.H{"error": "only OPEN food days can be deleted"})
		return
	}
	if errors.Is(err, core.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "food day not found"})
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
