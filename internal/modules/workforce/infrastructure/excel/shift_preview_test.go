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
					{MealType: "BREAKFAST", ServiceDate: date, Start: "06:00", End: "09:00"},
					{MealType: "LUNCH", ServiceDate: date, Start: "12:00", End: "15:00"},
				},
			},
			{
				ShiftType:     domain.ShiftNight,
				Worker:        domain.PreviewWorker{FullName: "Trabajador Noche", DocumentNumber: "****5678", EmployeeCode: "EMP-2"},
				AssignedMeals: []domain.PreviewMeal{{MealType: "DINNER", ServiceDate: date, Start: "20:00", End: "23:00"}},
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

func TestBuildShiftPreviewRangeCreatesOneSheetPerDay(t *testing.T) {
	first := time.Date(2027, 1, 4, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 1)
	report := &domain.ShiftPreviewRange{
		From: first,
		To:   second,
		Dates: []domain.ShiftPreview{
			{
				Date:    first,
				Summary: domain.ShiftPreviewSummary{ByMeal: map[string]int{"BREAKFAST": 1, "LUNCH": 1, "DINNER": 0}},
				Data: []domain.ShiftPreviewRow{{
					ShiftType: domain.ShiftDay,
					Worker:    domain.PreviewWorker{FullName: "Trabajador Uno", EmployeeCode: "EMP-1"},
					AssignedMeals: []domain.PreviewMeal{
						{MealType: "BREAKFAST", DisplayName: "DESAYUNO", ServiceDate: first, Start: "06:00", End: "09:00"},
						{MealType: "LUNCH", DisplayName: "ALMUERZO", ServiceDate: first, Start: "12:00", End: "15:00"},
					},
				}},
			},
			{Date: second, Summary: domain.ShiftPreviewSummary{ByMeal: map[string]int{"BREAKFAST": 0, "LUNCH": 0, "DINNER": 0}}, Data: []domain.ShiftPreviewRow{}},
		},
	}

	content, err := BuildShiftPreviewRange(report)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	file, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected open error: %v", err)
	}
	defer func() { _ = file.Close() }()

	wantSheets := []string{"2027-01-04", "2027-01-05"}
	if sheets := file.GetSheetList(); len(sheets) != len(wantSheets) || sheets[0] != wantSheets[0] || sheets[1] != wantSheets[1] {
		t.Fatalf("unexpected sheets: %v", sheets)
	}
	if breakfast, _ := file.GetCellValue("2027-01-04", "B2"); breakfast != "1" {
		t.Fatalf("breakfast summary = %q, want 1", breakfast)
	}
	if meal, _ := file.GetCellValue("2027-01-04", "G5"); meal != "DESAYUNO" {
		t.Fatalf("first meal = %q, want DESAYUNO", meal)
	}
	if title, _ := file.GetCellValue("2027-01-05", "A1"); title == "" {
		t.Fatal("empty dates must still have a report sheet")
	}
}
