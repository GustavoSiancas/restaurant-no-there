package excel

import (
	"fmt"
	"time"

	"backend/internal/modules/meals/domain"
	"github.com/xuri/excelize/v2"
)

func optionalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	hour := value.Hour()
	suffix := "a.m."
	if hour >= 12 {
		suffix = "p.m."
	}
	displayHour := hour % 12
	if displayHour == 0 {
		displayHour = 12
	}
	return fmt.Sprintf("%s %02d:%02d:%02d %s", value.Format("02/01/2006"), displayHour, value.Minute(), value.Second(), suffix)
}

func displayMeal(value domain.MealType) string {
	if value == domain.Lunch {
		return "ALMUERZO"
	}
	if value == domain.Dinner {
		return "CENA"
	}
	return "BREAKFAST"
}

func BuildMealStatusReport(report *domain.MealStatusReport) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	file.SetSheetName("Sheet1", "Resumen")
	if _, err := file.NewSheet("Detalle"); err != nil {
		return nil, err
	}
	headerStyle, err := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	_ = file.SetCellValue("Resumen", "A1", "REPORTE DE ESTADOS DE COMIDAS")
	_ = file.MergeCell("Resumen", "A1", "I1")
	_ = file.SetCellStyle("Resumen", "A1", "I1", headerStyle)
	_ = file.SetCellValue("Resumen", "A2", "Desde")
	_ = file.SetCellValue("Resumen", "B2", report.From.Format("2006-01-02"))
	_ = file.SetCellValue("Resumen", "C2", "Hasta")
	_ = file.SetCellValue("Resumen", "D2", report.To.Format("2006-01-02"))
	summaryHeaders := []string{"Comida", "Total", "Reclamadas", "No reclamadas", "CREATED", "CLAIMED", "CLAIMED_BUT_NOT_VALIDATED", "VALIDATED", "NOT_CLAIMED"}
	for column, header := range summaryHeaders {
		cell, _ := excelize.CoordinatesToCellName(column+1, 4)
		_ = file.SetCellValue("Resumen", cell, header)
	}
	_ = file.SetCellStyle("Resumen", "A4", "I4", headerStyle)
	for index, item := range report.Summary {
		values := []any{displayMeal(item.MealType), item.Total, item.Claimed, item.NotClaimed, item.ByStatus[domain.ClaimCreated], item.ByStatus[domain.ClaimClaimed], item.ByStatus[domain.ClaimClaimedNotValidated], item.ByStatus[domain.ClaimValidated], item.ByStatus[domain.ClaimNotClaimed]}
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, index+5)
			_ = file.SetCellValue("Resumen", cell, value)
		}
	}
	for column, width := range map[string]float64{"A": 18, "B": 12, "C": 14, "D": 16, "E": 14, "F": 14, "G": 30, "H": 14, "I": 18} {
		_ = file.SetColWidth("Resumen", column, column, width)
	}

	headers := []string{"UUID", "Fecha", "Comida", "Estado", "Trabajador", "DNI", "Código", "Departamento", "Hora de reclamo", "Hora de validación", "Worker UUID"}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		_ = file.SetCellValue("Detalle", cell, header)
	}
	lastHeader, _ := excelize.CoordinatesToCellName(len(headers), 1)
	_ = file.SetCellStyle("Detalle", "A1", lastHeader, headerStyle)
	for index, item := range report.Data {
		values := []any{item.ID, item.ServiceDate.Format("2006-01-02"), displayMeal(item.MealType), string(item.Status), item.FullName, item.DocumentNumber, item.EmployeeCode, optionalText(item.Department), optionalTime(item.ClaimedAt), optionalTime(item.ValidatedAt), item.WorkerID}
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, index+2)
			_ = file.SetCellValue("Detalle", cell, value)
		}
	}
	for column, width := range map[string]float64{"A": 38, "B": 13, "C": 16, "D": 30, "E": 30, "F": 14, "G": 16, "H": 22, "I": 24, "J": 24, "K": 38} {
		_ = file.SetColWidth("Detalle", column, column, width)
	}
	_ = file.SetPanes("Detalle", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	file.SetActiveSheet(0)
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return buffer.Bytes(), nil
}
