package http

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "backend/internal/core/domain"
	"backend/internal/modules/workforce/application"
	"backend/internal/modules/workforce/domain"
	previewexcel "backend/internal/modules/workforce/infrastructure/excel"
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

func previewQuery(c *gin.Context) (
	time.Time,
	[]string,
	int,
	int,
	error,
) {
	var date time.Time
	var err error

	if value := strings.TrimSpace(c.Query("date")); value != "" {
		date, err = time.Parse("2006-01-02", value)
		if err != nil {
			return date, nil, 0, 0, fmt.Errorf("date must use YYYY-MM-DD")
		}
	}

	page, pageSize := 1, 20

	if value := c.Query("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return date, nil, 0, 0, fmt.Errorf("page must be positive")
		}
	}

	if value := c.Query("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 {
			return date, nil, 0, 0, fmt.Errorf("page_size must be positive")
		}
	}

	return date, c.QueryArray("meal_type"), page, pageSize, nil
}

func (h *Handler) ShiftPreview(c *gin.Context) {
	fromValue := strings.TrimSpace(c.Query("from"))
	toValue := strings.TrimSpace(c.Query("to"))
	if fromValue != "" || toValue != "" {
		if fromValue == "" || toValue == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are both required"})
			return
		}
		from, fromErr := time.Parse("2006-01-02", fromValue)
		to, toErr := time.Parse("2006-01-02", toValue)
		if fromErr != nil || toErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from and to must use YYYY-MM-DD"})
			return
		}
		report, err := h.service.ShiftPreviewRange(c, from, to, c.QueryArray("meal_type"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, report)
		return
	}

	date, meals, page, size, err := previewQuery(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	report, err := h.service.ShiftPreview(
		c,
		date,
		meals,
		page,
		size,
		true,
	)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, report)
}

func (h *Handler) ExportShiftPreview(c *gin.Context) {
	fromValue := strings.TrimSpace(c.Query("from"))
	toValue := strings.TrimSpace(c.Query("to"))
	if fromValue != "" || toValue != "" {
		if fromValue == "" || toValue == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are both required"})
			return
		}
		from, fromErr := time.Parse("2006-01-02", fromValue)
		to, toErr := time.Parse("2006-01-02", toValue)
		if fromErr != nil || toErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from and to must use YYYY-MM-DD"})
			return
		}
		report, err := h.service.ShiftPreviewRange(c, from, to, c.QueryArray("meal_type"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		content, err := previewexcel.BuildShiftPreviewRange(report)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate Excel report"})
			return
		}
		name := fmt.Sprintf("comidas-%s-%s.xlsx", from.Format("20060102"), to.Format("20060102"))
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
		return
	}

	date, _, _, _, err := previewQuery(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	report, err := h.service.ShiftPreview(
		c,
		date,
		nil,
		1,
		20,
		false,
	)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	content, err := previewexcel.BuildShiftPreview(report)
	if err != nil {
		c.JSON(500, gin.H{"error": "could not generate Excel report"})
		return
	}

	name := "Comidas Dia Actual.xlsx"

	c.Header(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, name),
	)

	c.Data(
		200,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		content,
	)
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
		writeAssignmentError(c, err)
		return
	}
	c.JSON(http.StatusCreated, a)
}

type massiveAssignmentRequest struct {
	Dates struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"dates"`
	Shift   domain.ShiftType `json:"shift"`
	Workers []string         `json:"workers"`
}

func (h *Handler) AddMassiveShiftWorkers(c *gin.Context) {
	var r massiveAssignmentRequest
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	from, fromErr := time.Parse("2006-01-02", r.Dates.From)
	to, toErr := time.Parse("2006-01-02", r.Dates.To)
	if fromErr != nil || toErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dates.from and dates.to must use YYYY-MM-DD"})
		return
	}
	assignedBy, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user not found"})
		return
	}
	result, err := h.service.AddMassiveShiftWorkers(c, r.Workers, r.Shift, assignedBy.(string), from, to)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, core.ErrConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
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
		var conflict *domain.AssignmentConflictError
		if errors.As(err, &conflict) {
			writeAssignmentError(c, err)
			return
		}
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

func (h *Handler) DeleteAssignment(c *gin.Context) {
	err := h.service.DeleteAssignment(c, c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		message := "could not delete assignment"
		if errors.Is(err, core.ErrNotFound) {
			status = http.StatusNotFound
			message = "assignment not found"
		} else if errors.Is(err, core.ErrLocked) {
			status = http.StatusLocked
			message = err.Error()
		}
		c.JSON(status, gin.H{"error": gin.H{"code": "ASSIGNMENT_DELETE_REJECTED", "message": message}})
		return
	}
	c.Status(http.StatusNoContent)
}

func writeAssignmentError(c *gin.Context, err error) {
	var conflict *domain.AssignmentConflictError
	if errors.As(err, &conflict) {
		existing := conflict.Existing
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "WORKER_ALREADY_ASSIGNED", "message": "El trabajador ya tiene un turno asignado para esa fecha", "worker_id": existing.WorkerID, "work_date": existing.WorkDate.Format("2006-01-02"), "existing_shift": gin.H{"assignment_id": existing.ID, "shift_type": existing.ShiftType, "notes": existing.Notes}}})
		return
	}
	status := http.StatusBadRequest
	if errors.Is(err, core.ErrConflict) {
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"error": gin.H{"code": "ASSIGNMENT_REJECTED", "message": err.Error()}})
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

func (h *Handler) ListWorkerAssignments(c *gin.Context) {
	items, err := h.service.ListWorkerAssignments(c, c.Param("id"), c.Query("period"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) ListWorkerAssignmentsRange(c *gin.Context) {
	from, e1 := time.Parse("2006-01-02", c.Query("from"))
	to, e2 := time.Parse("2006-01-02", c.Query("to"))
	if e1 != nil || e2 != nil {
		c.JSON(400, gin.H{"error": "from and to must use YYYY-MM-DD"})
		return
	}
	items, err := h.service.ListWorkerAssignmentsRange(c, c.Param("id"), from, to)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) ListMyAssignmentsRange(c *gin.Context) {
	workerID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user not found"})
		return
	}
	from, e1 := time.Parse("2006-01-02", c.Query("from"))
	to, e2 := time.Parse("2006-01-02", c.Query("to"))
	if e1 != nil || e2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to must use YYYY-MM-DD"})
		return
	}
	items, err := h.service.ListWorkerAssignmentsRange(c, workerID.(string), from, to)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}
