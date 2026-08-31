package excel

import (
	"backend/internal/modules/workforce/domain"
	"bytes"
	"github.com/xuri/excelize/v2"
)

func BuildShiftPreview(report *domain.ShiftPreview) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Turnos"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"Trabajador", "Documento", "Código", "Cargo", "Departamento", "Turno", "Fecha", "Comidas asignadas"}
	for i, v := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, v)
	}
	for i, row := range report.Data {
		meals := ""
		for j, m := range row.AssignedMeals {
			if j > 0 {
				meals += " | "
			}
			meals += m.DisplayName + " " + m.Start + "-" + m.End
		}
		values := []any{row.Worker.FullName, row.Worker.DocumentNumber, row.Worker.EmployeeCode, text(row.Worker.JobTitle), text(row.Worker.Department), row.ShiftType, row.WorkDate.Format("2006-01-02"), meals}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, i+2)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	buffer := bytes.NewBuffer(nil)
	if err := f.Write(buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
