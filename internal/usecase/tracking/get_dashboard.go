package tracking

import (
	"fmt"
	"time"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// GetDashboardOptions holds options for the GetDashboard use case.
type GetDashboardOptions struct {
	Start     time.Time
	End       time.Time
	ActiveMin int
	TopN      int
}

// GetDashboard retrieves dashboard metrics for a tenant.
func GetDashboard(repo tracking.Repository, tenantID string, opts GetDashboardOptions) (*tracking.DashboardMetrics, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID required")
	}

	if opts.ActiveMin <= 0 {
		opts.ActiveMin = 30
	}
	if opts.TopN <= 0 {
		opts.TopN = 10
	}

	now := time.Now()
	if opts.Start.IsZero() || opts.End.IsZero() {
		opts.Start = now.AddDate(0, 0, -30)
		opts.End = now
	}

	dateRange := tracking.DateRange{
		Start: opts.Start,
		End:   opts.End,
	}

	metrics := &tracking.DashboardMetrics{}

	var err error

	metrics.TopPages, err = repo.GetTopPages(tenantID, dateRange, opts.TopN)
	if err != nil {
		return nil, fmt.Errorf("get top pages: %w", err)
	}

	metrics.TopReferrers, err = repo.GetTopReferrers(tenantID, dateRange, opts.TopN)
	if err != nil {
		return nil, fmt.Errorf("get top referrers: %w", err)
	}

	metrics.DeviceBreakdown, err = repo.GetDeviceBreakdown(tenantID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get device breakdown: %w", err)
	}

	metrics.BrowserBreakdown, err = repo.GetBrowserBreakdown(tenantID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get browser breakdown: %w", err)
	}

	metrics.ActiveVisitors, err = repo.GetActiveVisitors(tenantID, opts.ActiveMin)
	if err != nil {
		return nil, fmt.Errorf("get active visitors: %w", err)
	}

	metrics, err = repo.GetDashboardMetrics(tenantID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get dashboard metrics: %w", err)
	}

	return metrics, nil
}
