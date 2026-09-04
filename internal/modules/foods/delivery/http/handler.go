package http

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/foods/application"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct{ service *application.Service }

func New(service *application.Service) *Handler { return &Handler{service: service} }
func respondError(c *gin.Context, err error) {
	status, message := http.StatusInternalServerError, "could not process request"
	if errors.Is(err, core.ErrInvalidInput) {
		status, message = http.StatusBadRequest, err.Error()
	}
	if errors.Is(err, core.ErrConflict) {
		status, message = http.StatusConflict, "tag name already exists"
	}
	if errors.Is(err, core.ErrNotFound) {
		status, message = http.StatusNotFound, "food not found"
	}
	c.JSON(status, gin.H{"error": message})
}
func (h *Handler) CreateFood(c *gin.Context) {
	var in application.CreateFoodInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	food, err := h.service.CreateFood(c, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, food)
}

func (h *Handler) GetFood(c *gin.Context) {
	food, err := h.service.GetFood(c, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, food)
}

func (h *Handler) UpdateFood(c *gin.Context) {
	var in application.UpdateFoodInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	food, err := h.service.UpdateFood(c, c.Param("id"), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, food)
}
func (h *Handler) ListFoods(c *gin.Context) {
	page, e1 := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, e2 := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if e1 != nil || e2 != nil {
		c.JSON(400, gin.H{"error": "page and page_size must be integers"})
		return
	}
	ids := []string{}
	for _, value := range c.QueryArray("tag_ids") {
		if strings.TrimSpace(value) != "" {
			ids = append(ids, strings.Split(value, ",")...)
		}
	}
	result, err := h.service.ListFoods(c, page, size, ids)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *Handler) CreateTag(c *gin.Context) {
	var in struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	tag, err := h.service.CreateTag(c, in.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tag)
}
func (h *Handler) ListTags(c *gin.Context) {
	tags, err := h.service.ListTags(c)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tags)
}
