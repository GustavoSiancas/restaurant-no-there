package excel

import (
	"backend/internal/modules/workforce/domain"
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

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
