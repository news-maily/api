package reports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mailbadger/app/entities"
	"github.com/mailbadger/app/services/exporters"
	"github.com/mailbadger/app/storage"
)

const (
	reportTypeExport = "export"
	maxReports       = 100
)

var (
	ErrAnotherReportRunning = errors.New("another report running")
	ErrLimitReached         = errors.New("report limit reached")
)

// ReportService represents all report functionalities
type ReportService interface {
	GenerateExportReport(c context.Context, userID int64, report *entities.Report) (*entities.Report, error)
	CreateExportReport(c context.Context, userID int64, resource string, note string, date time.Time) (*entities.Report, error)
}

type reportService struct {
	exporter exporters.Exporter
}

// NewReportService represents constructor for ReportService
func NewReportService(exporter exporters.Exporter) ReportService {
	return &reportService{
		exporter: exporter,
	}
}

// GenerateExportReport starts the resources export method
func (r *reportService) GenerateExportReport(c context.Context, userID int64, report *entities.Report) (*entities.Report, error) {
	var updateErr error
	err := r.exporter.Export(c, userID, report)
	if err != nil {
		// report failed
		report.Status = entities.StatusFailed

		updateErr = storage.UpdateReport(c, report)
		if updateErr != nil {
			return nil, fmt.Errorf("update report: %w", updateErr)
		}

		return nil, err
	}

	// report generated successfully
	report.Status = entities.StatusDone

	// updating the report
	updateErr = storage.UpdateReport(c, report)
	if updateErr != nil {
		return nil, fmt.Errorf("update report: %w", updateErr)
	}

	return report, nil
}

// CreateExportReport creates export report
func (r *reportService) CreateExportReport(c context.Context, userID int64, resource, note string, date time.Time) (*entities.Report, error) {
	if isAnotherReportRunning(c, userID) {
		return nil, ErrAnotherReportRunning
	}

	limit, err := isLimitExceeded(c, userID, date)
	if err != nil {
		return nil, fmt.Errorf("is limit exceeded check error: %w", err)
	}
	if limit {
		return nil, ErrLimitReached
	}

	report := &entities.Report{
		UserID:   userID,
		Resource: resource,
		FileName: generateFilename(resource, date),
		Type:     reportTypeExport,
		Status:   entities.StatusInProgress,
		Note:     note,
	}

	err = storage.CreateReport(c, report)
	if err != nil {
		return nil, fmt.Errorf("create report: %w", err)
	}

	return report, nil
}

// generateFilename generates the report filename
func generateFilename(resource string, date time.Time) string {
	return fmt.Sprintf("%s_%d.csv", resource, date.Unix())
}

// isAnotherReportRunning returns true if there is report in progress for a user or false if all are done
func isAnotherReportRunning(c context.Context, userID int64) bool {
	_, err := storage.GetRunningReportForUser(c, userID)
	return err == nil
}

// isLimitExceeded returns true if there are less than maxReports reports per day
func isLimitExceeded(c context.Context, userID int64, time time.Time) (bool, error) {
	n, err := storage.GetNumberOfReportsForDate(c, userID, time)
	if err != nil {
		return false, err
	}

	return n > maxReports, nil
}
