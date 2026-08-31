package excel

import (
	"backend/internal/modules/workforce/domain"
	"bytes"
	"strconv"

	"github.com/xuri/excelize/v2"
)

type mealSheet struct {
	name        string
	mealType    string
	displayName string
}

func BuildShiftPreview(report *domain.ShiftPreview) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheets := []mealSheet{
		{name: "Desayunos", mealType: "DESAYUNO", displayName: "DESAYUNOS"},
		{name: "Almuerzos", mealType: "TARDE", displayName: "ALMUERZOS"},
		{name: "Cenas", mealType: "NOCHE", displayName: "CENAS"},
	}
	f.SetSheetName("Sheet1", sheets[0].name)
	for _, definition := range sheets[1:] {
		if _, err := f.NewSheet(definition.name); err != nil {
			return nil, err
		}
	}

	titleStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 16}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	summaryStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "1F1F1F"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	if err != nil {
		return nil, err
	}

	for _, definition := range sheets {
		if err := buildMealSheet(f, report, definition, titleStyle, summaryStyle, headerStyle); err != nil {
			return nil, err
		}
	}
	f.SetActiveSheet(0)

	buffer := bytes.NewBuffer(nil)
	if err := f.Write(buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildMealSheet(f *excelize.File, report *domain.ShiftPreview, definition mealSheet, titleStyle, summaryStyle, headerStyle int) error {
	type assignedRow struct {
		row  domain.ShiftPreviewRow
		meal domain.PreviewMeal
	}
	rows := make([]assignedRow, 0)
	for _, row := range report.Data {
		for _, meal := range row.AssignedMeals {
			if meal.MealType == definition.mealType {
				rows = append(rows, assignedRow{row: row, meal: meal})
			}
		}
	}

	sheet := definition.name
	if err := f.MergeCell(sheet, "A1", "H1"); err != nil {
		return err
	}
	_ = f.SetCellValue(sheet, "A1", "COMIDAS DÍA ACTUAL - "+definition.displayName)
	_ = f.SetCellStyle(sheet, "A1", "H1", titleStyle)
	_ = f.SetRowHeight(sheet, 1, 28)
	_ = f.SetCellValue(sheet, "A2", "Fecha de comida")
	_ = f.SetCellValue(sheet, "B2", report.Date.Format("2006-01-02"))
	_ = f.SetCellValue(sheet, "G2", "Cantidad")
	_ = f.SetCellValue(sheet, "H2", len(rows))
	_ = f.SetCellStyle(sheet, "A2", "H2", summaryStyle)

	headers := []string{"Trabajador", "Documento", "Código", "Cargo", "Departamento", "Turno", "Fecha de comida", "Horario"}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 4)
		_ = f.SetCellValue(sheet, cell, header)
	}
	_ = f.SetCellStyle(sheet, "A4", "H4", headerStyle)
	_ = f.SetRowHeight(sheet, 4, 22)

	for index, item := range rows {
		values := []any{item.row.Worker.FullName, item.row.Worker.DocumentNumber, item.row.Worker.EmployeeCode, text(item.row.Worker.JobTitle), text(item.row.Worker.Department), item.row.ShiftType, item.meal.ServiceDate.Format("2006-01-02"), item.meal.Start + " - " + item.meal.End}
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, index+5)
			_ = f.SetCellValue(sheet, cell, value)
		}
	}

	widths := map[string]float64{"A": 30, "B": 16, "C": 14, "D": 25, "E": 22, "F": 12, "G": 18, "H": 16}
	for column, width := range widths {
		_ = f.SetColWidth(sheet, column, column, width)
	}
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})
	if len(rows) > 0 {
		_ = f.AutoFilter(sheet, "A4:H"+strconv.Itoa(len(rows)+4), []excelize.AutoFilterOptions{})
	}
	return nil
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
