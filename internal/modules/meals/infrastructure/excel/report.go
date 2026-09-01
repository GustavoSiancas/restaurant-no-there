package excel

import (
	"fmt"
	"time"

	"backend/internal/modules/meals/domain"
	"github.com/xuri/excelize/v2"
)

func BuildDetailedReport(report *domain.DetailedReport) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	const detailSheet = "Detalle"
	const summarySheet = "Resumen"
	file.SetSheetName("Sheet1", summarySheet)
	if _, err := file.NewSheet(detailSheet); err != nil {
		return nil, err
	}
	headerStyle, err := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	dateStyle, err := file.NewStyle(&excelize.Style{NumFmt: 14})
	if err != nil {
		return nil, err
	}
	summaryRows := [][]any{
		{"REPORTE DE ALIMENTACIÓN"},
		{"Desde", report.Filters.From.Format("02/01/2006")},
		{"Hasta", report.Filters.To.Format("02/01/2006")},
		{"Comida", valueOrAll(string(report.Filters.MealType))},
		{"Turno", valueOrAll(report.Filters.ShiftType)},
		{},
		{"Métrica", "Cantidad"},
		{"Total elegibles", report.Summary.TotalEligible},
		{"Comieron / entregados", report.Summary.Consumed},
		{"Pidieron sin validar", report.Summary.RequestedNotValidated},
		{"No reclamaron", report.Summary.NotClaimed},
		{"No consumieron", report.Summary.DidNotConsume},
	}
	for rowIndex, row := range summaryRows {
		for columnIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err = file.SetCellValue(summarySheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	_ = file.MergeCell(summarySheet, "A1", "B1")
	_ = file.SetCellStyle(summarySheet, "A1", "B1", headerStyle)
	_ = file.SetCellStyle(summarySheet, "A7", "B7", headerStyle)
	_ = file.SetColWidth(summarySheet, "A", "A", 28)
	_ = file.SetColWidth(summarySheet, "B", "B", 22)

	headers := []string{"UUID", "Fecha", "Comida", "Turno", "Resultado", "Trabajador", "DNI", "Código", "Departamento", "Hora de pedido", "Hora de entrega", "Worker UUID"}
	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		_ = file.SetCellValue(detailSheet, cell, header)
	}
	lastHeader, _ := excelize.CoordinatesToCellName(len(headers), 1)
	_ = file.SetCellStyle(detailSheet, "A1", lastHeader, headerStyle)
	for index, item := range report.Data {
		row := index + 2
		values := []any{item.ID, item.ServiceDate, displayMeal(item.MealType), item.ShiftType, displayStatus(item.Status), item.FullName, item.DocumentNumber, item.EmployeeCode, optionalText(item.Department), optionalTime(item.ClaimedAt), optionalTime(item.ValidatedAt), item.WorkerID}
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, row)
			_ = file.SetCellValue(detailSheet, cell, value)
		}
		dateCell, _ := excelize.CoordinatesToCellName(2, row)
		_ = file.SetCellStyle(detailSheet, dateCell, dateCell, dateStyle)
	}
	widths := map[string]float64{"A": 38, "B": 13, "C": 16, "D": 11, "E": 22, "F": 30, "G": 14, "H": 16, "I": 22, "J": 22, "K": 22, "L": 38}
	for column, width := range widths {
		_ = file.SetColWidth(detailSheet, column, column, width)
	}
	_ = file.SetPanes(detailSheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	file.SetActiveSheet(0)
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return buffer.Bytes(), nil
}

func valueOrAll(value string) string {
	if value == "" {
		return "TODOS"
	}
	return value
}
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
func displayStatus(value domain.ClaimStatus) string {
	switch value {
	case domain.ClaimValidated:
		return "CONSUMIÓ"
	case domain.ClaimClaimed:
		return "SOLICITADO"
	case domain.ClaimClaimedNotValidated:
		return "SOLICITADO - NO VALIDADO"
	default:
		return "NO CONSUMIÓ"
	}
}
