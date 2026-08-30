package http

import (
	"errors"
	"net/http"
	"time"

	core "backend/internal/core/domain"
	"backend/internal/modules/workforce/application"
	"backend/internal/modules/workforce/domain"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *application.Service }

func New(service *application.Service) *Handler { return &Handler{service: service} }

type registerWorkerRequest struct {
	DNI                   string `json:"dni"`
	Email                 string `json:"email"`
	FirstName             string `json:"first_name"`
	LastName              string `json:"last_name"`
	EmployeeCode          string `json:"employee_code"`
	JobTitle              string `json:"job_title"`
	Department            string `json:"department"`
	Phone                 string `json:"phone"`
	Address               string `json:"address"`
	HireDate              string `json:"hire_date"`
	EmergencyContactName  string `json:"emergency_contact_name"`
	EmergencyContactPhone string `json:"emergency_contact_phone"`
	Notes                 string `json:"notes"`
}

func (h *Handler) RegisterWorker(c *gin.Context) {
	var r registerWorkerRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	var hireDate *time.Time
	if r.HireDate != "" {
		parsed, err := time.Parse("2006-01-02", r.HireDate)
		if err != nil {
			c.JSON(400, gin.H{"error": "hire_date must use YYYY-MM-DD"})
			return
		}
		hireDate = &parsed
	}
	user, info, err := h.service.RegisterWorker(c, application.RegisterWorkerInput{DNI: r.DNI, Email: r.Email, FirstName: r.FirstName, LastName: r.LastName, EmployeeCode: r.EmployeeCode, JobTitle: r.JobTitle, Department: r.Department, Phone: r.Phone, Address: r.Address, HireDate: hireDate, EmergencyContactName: r.EmergencyContactName, EmergencyContactPhone: r.EmergencyContactPhone, Notes: r.Notes})
	if err != nil {
		status := 400
		if errors.Is(err, core.ErrConflict) {
			status = 409
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": user, "worker_information": info})
}

type assignmentRequest struct {
	WorkerID  string           `json:"worker_id"`
	ShiftType domain.ShiftType `json:"shift_type"`
	WorkDate  string           `json:"work_date"`
	Notes     string           `json:"notes"`
}

func (h *Handler) AssignWorker(c *gin.Context) {
	var r assignmentRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	date, err := time.Parse("2006-01-02", r.WorkDate)
	if err != nil {
		c.JSON(400, gin.H{"error": "work_date must use YYYY-MM-DD"})
		return
	}
	assignedBy, ok := c.Get("user_id")
	if !ok {
		c.JSON(401, gin.H{"error": "authenticated user not found"})
		return
	}
	a, err := h.service.AssignWorker(c, r.WorkerID, r.ShiftType, assignedBy.(string), date, r.Notes)
	if err != nil {
		status := 400
		if errors.Is(err, core.ErrConflict) {
			status = 409
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, a)
}

type updateAssignmentRequest struct {
	ShiftType domain.ShiftType `json:"shift_type"`
	WorkDate  string           `json:"work_date"`
	Notes     string           `json:"notes"`
}

func (h *Handler) UpdateAssignment(c *gin.Context) {
	var r updateAssignmentRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	date, err := time.Parse("2006-01-02", r.WorkDate)
	if err != nil {
		c.JSON(400, gin.H{"error": "work_date must use YYYY-MM-DD"})
		return
	}
	assignment, err := h.service.UpdateAssignment(c, c.Param("id"), r.ShiftType, date, r.Notes)
	if err != nil {
		status := 400
		if errors.Is(err, core.ErrNotFound) {
			status = 404
		} else if errors.Is(err, core.ErrConflict) {
			status = 409
		} else if errors.Is(err, core.ErrLocked) {
			status = 423
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, assignment)
}
func (h *Handler) ListAssignments(c *gin.Context) {
	from, e1 := time.Parse("2006-01-02", c.Query("from"))
	to, e2 := time.Parse("2006-01-02", c.Query("to"))
	if e1 != nil || e2 != nil {
		c.JSON(400, gin.H{"error": "from and to must use YYYY-MM-DD"})
		return
	}
	items, err := h.service.ListAssignments(c, from, to)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, items)
}
