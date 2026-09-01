package excel

import (
	"bytes"
	"testing"
	"time"

	"backend/internal/modules/meals/domain"
	"github.com/xuri/excelize/v2"
)

func TestBuildDetailedReportCreatesSummaryAndDetailSheets(t *testing.T) {
	date := time.Date(2026, 9, 6, 0, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60))
	report := &domain.DetailedReport{
		Filters: domain.ReportFilters{From: date, To: date},
		Summary: domain.DetailedReportSummary{TotalEligible: 1, NotClaimed: 1, DidNotConsume: 1},
		Data:    []domain.DetailedReportRow{{ID: "claim", ServiceDate: date, MealType: domain.Lunch, ShiftType: "DAY", Status: domain.ClaimNotClaimed, WorkerID: "worker", FullName: "María Pérez", DocumentNumber: "****5678", EmployeeCode: "EMP-001"}},
	}
	content, err := BuildDetailedReport(report)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("generated content is not a valid xlsx: %v", err)
	}
	defer func() { _ = workbook.Close() }()
	_, summaryErr := workbook.GetSheetIndex("Resumen")
	_, detailErr := workbook.GetSheetIndex("Detalle")
	if summaryErr != nil || detailErr != nil {
		t.Fatalf("expected Resumen and Detalle sheets, got %v", workbook.GetSheetList())
	}
}
