package excel

import (
	"backend/internal/modules/workforce/domain"
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestBuildShiftPreviewSeparatesMealsIntoSheets(t *testing.T) {
	date := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	report := &domain.ShiftPreview{
		Date: date,
		Data: []domain.ShiftPreviewRow{
			{
				ShiftType: domain.ShiftDay,
				Worker:    domain.PreviewWorker{FullName: "Trabajador Día", DocumentNumber: "****1234", EmployeeCode: "EMP-1"},
				AssignedMeals: []domain.PreviewMeal{
					{MealType: "DESAYUNO", ServiceDate: date, Start: "06:00", End: "10:00"},
					{MealType: "TARDE", ServiceDate: date, Start: "12:00", End: "15:00"},
				},
			},
			{
				ShiftType:     domain.ShiftNight,
				Worker:        domain.PreviewWorker{FullName: "Trabajador Noche", DocumentNumber: "****5678", EmployeeCode: "EMP-2"},
				AssignedMeals: []domain.PreviewMeal{{MealType: "NOCHE", ServiceDate: date, Start: "18:00", End: "22:00"}},
			},
		},
	}

	content, err := BuildShiftPreview(report)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	file, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected open error: %v", err)
	}
	defer func() { _ = file.Close() }()

	wantSheets := []string{"Desayunos", "Almuerzos", "Cenas"}
	if sheets := file.GetSheetList(); len(sheets) != len(wantSheets) {
		t.Fatalf("unexpected sheets: %v", sheets)
	} else {
		for index, want := range wantSheets {
			if sheets[index] != want {
				t.Fatalf("sheet %d = %q, want %q", index, sheets[index], want)
			}
		}
	}
	for _, sheet := range wantSheets {
		if quantity, _ := file.GetCellValue(sheet, "H2"); quantity != "1" {
			t.Fatalf("%s quantity = %q, want 1", sheet, quantity)
		}
		if worker, _ := file.GetCellValue(sheet, "A5"); worker == "" {
			t.Fatalf("%s must contain its assigned worker", sheet)
		}
	}
}
