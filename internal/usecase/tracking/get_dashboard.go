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

// GetDashboard retrieves dashboard metrics for a context.
func GetDashboard(repo tracking.Repository, contextID string, opts GetDashboardOptions) (*tracking.DashboardMetrics, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
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

	metrics.TopPages, err = repo.GetTopPages(contextID, dateRange, opts.TopN)
	if err != nil {
		return nil, fmt.Errorf("get top pages: %w", err)
	}

	metrics.TopReferrers, err = repo.GetTopReferrers(contextID, dateRange, opts.TopN)
	if err != nil {
		return nil, fmt.Errorf("get top referrers: %w", err)
	}

	metrics.DeviceBreakdown, err = repo.GetDeviceBreakdown(contextID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get device breakdown: %w", err)
	}

	metrics.BrowserBreakdown, err = repo.GetBrowserBreakdown(contextID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get browser breakdown: %w", err)
	}

	metrics.ActiveVisitors, err = repo.GetActiveVisitors(contextID, opts.ActiveMin)
	if err != nil {
		return nil, fmt.Errorf("get active visitors: %w", err)
	}

	metrics, err = repo.GetDashboardMetrics(contextID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get dashboard metrics: %w", err)
	}

	return metrics, nil
}
