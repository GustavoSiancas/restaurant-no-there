package excel

import (
	"backend/internal/modules/workforce/domain"
	"bytes"
	"strconv"

	"github.com/xuri/excelize/v2"
)

func BuildShiftPreviewRange(report *domain.ShiftPreviewRange) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	titleStyle, summaryStyle, headerStyle, err := previewStyles(f)
	if err != nil {
		return nil, err
	}
	for index := range report.Dates {
		preview := &report.Dates[index]
		sheet := preview.Date.Format("2006-01-02")
		if index == 0 {
			f.SetSheetName("Sheet1", sheet)
		} else if _, err = f.NewSheet(sheet); err != nil {
			return nil, err
		}
		if err = buildDailySheet(f, sheet, preview, titleStyle, summaryStyle, headerStyle); err != nil {
			return nil, err
		}
	}
	if len(report.Dates) == 0 {
		f.SetSheetName("Sheet1", "Sin datos")
	}
	f.SetActiveSheet(0)
	buffer := bytes.NewBuffer(nil)
	if err = f.Write(buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func previewStyles(f *excelize.File) (titleStyle, summaryStyle, headerStyle int, err error) {
	titleStyle, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 16}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	if err != nil {
		return
	}
	summaryStyle, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "1F1F1F"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "center"}})
	if err != nil {
		return
	}
	headerStyle, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	return
}

func buildDailySheet(f *excelize.File, sheet string, report *domain.ShiftPreview, titleStyle, summaryStyle, headerStyle int) error {
	type dailyRow struct {
		row  domain.ShiftPreviewRow
		meal domain.PreviewMeal
	}
	rows := make([]dailyRow, 0)
	for _, row := range report.Data {
		for _, meal := range row.AssignedMeals {
			rows = append(rows, dailyRow{row: row, meal: meal})
		}
	}
	if err := f.MergeCell(sheet, "A1", "I1"); err != nil {
		return err
	}
	_ = f.SetCellValue(sheet, "A1", "COMIDAS A PREPARAR - "+report.Date.Format("2006-01-02"))
	_ = f.SetCellStyle(sheet, "A1", "I1", titleStyle)
	_ = f.SetRowHeight(sheet, 1, 28)
	_ = f.SetCellValue(sheet, "A2", "Desayunos")
	_ = f.SetCellValue(sheet, "B2", report.Summary.ByMeal["BREAKFAST"])
	_ = f.SetCellValue(sheet, "C2", "Almuerzos")
	_ = f.SetCellValue(sheet, "D2", report.Summary.ByMeal["LUNCH"])
	_ = f.SetCellValue(sheet, "E2", "Cenas")
	_ = f.SetCellValue(sheet, "F2", report.Summary.ByMeal["DINNER"])
	_ = f.SetCellValue(sheet, "G2", "Total")
	_ = f.SetCellValue(sheet, "H2", len(rows))
	_ = f.SetCellStyle(sheet, "A2", "I2", summaryStyle)

	headers := []string{"Trabajador", "Documento", "Código", "Cargo", "Departamento", "Turno", "Comida", "Fecha de comida", "Horario"}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 4)
		_ = f.SetCellValue(sheet, cell, header)
	}
	_ = f.SetCellStyle(sheet, "A4", "I4", headerStyle)
	_ = f.SetRowHeight(sheet, 4, 22)
	for index, item := range rows {
		values := []any{item.row.Worker.FullName, item.row.Worker.DocumentNumber, item.row.Worker.EmployeeCode, text(item.row.Worker.JobTitle), text(item.row.Worker.Department), item.row.ShiftType, item.meal.DisplayName, item.meal.ServiceDate.Format("2006-01-02"), item.meal.Start + " - " + item.meal.End}
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, index+5)
			_ = f.SetCellValue(sheet, cell, value)
		}
	}
	widths := map[string]float64{"A": 30, "B": 16, "C": 14, "D": 25, "E": 22, "F": 12, "G": 16, "H": 18, "I": 16}
	for column, width := range widths {
		_ = f.SetColWidth(sheet, column, column, width)
	}
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})
	if len(rows) > 0 {
		_ = f.AutoFilter(sheet, "A4:I"+strconv.Itoa(len(rows)+4), []excelize.AutoFilterOptions{})
	}
	return nil
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
