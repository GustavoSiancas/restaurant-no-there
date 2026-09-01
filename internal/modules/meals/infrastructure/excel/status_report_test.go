package excel

import (
	"bytes"
	"testing"
	"time"

	"backend/internal/modules/meals/domain"
	"github.com/xuri/excelize/v2"
)

func TestBuildMealStatusReportCreatesSummaryAndDetail(t *testing.T) {
	date := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	report := &domain.MealStatusReport{
		From: date, To: date,
		Summary: []domain.MealStatusSummary{{MealType: domain.Breakfast, Total: 2, Claimed: 1, NotClaimed: 1, ByStatus: map[domain.ClaimStatus]int64{domain.ClaimValidated: 1, domain.ClaimNotClaimed: 1}}},
		Data:    []domain.DetailedReportRow{{ID: "claim", ServiceDate: date, MealType: domain.Breakfast, Status: domain.ClaimValidated, FullName: "Trabajador"}},
	}
	content, err := BuildMealStatusReport(report)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("invalid xlsx: %v", err)
	}
	defer func() { _ = workbook.Close() }()
	if sheets := workbook.GetSheetList(); len(sheets) != 2 || sheets[0] != "Resumen" || sheets[1] != "Detalle" {
		t.Fatalf("unexpected sheets: %v", sheets)
	}
	if claimed, _ := workbook.GetCellValue("Resumen", "C5"); claimed != "1" {
		t.Fatalf("claimed = %q, want 1", claimed)
	}
	if status, _ := workbook.GetCellValue("Detalle", "D2"); status != "VALIDATED" {
		t.Fatalf("status = %q, want VALIDATED", status)
	}
}
