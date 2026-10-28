package tracking

import (
	"fmt"
	"time"

	"github.com/h2hsecure/automata/internal/domain/tracking"
	"github.com/h2hsecure/automata/pkg/ua"
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

	metrics.ActiveVisitors, err = repo.GetActiveVisitors(contextID, opts.ActiveMin)
	if err != nil {
		return nil, fmt.Errorf("get active visitors: %w", err)
	}

	dash, err := repo.GetDashboardMetrics(contextID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get dashboard metrics: %w", err)
	}
	metrics.TotalVisitors = dash.TotalVisitors
	metrics.PageViews = dash.PageViews

	metrics.DeviceBreakdown = make(map[string]int)
	metrics.BrowserBreakdown = make(map[string]int)
	groups, err := repo.GetUAGroups(contextID, dateRange)
	if err != nil {
		return nil, fmt.Errorf("get ua groups: %w", err)
	}
	for _, g := range groups {
		device, browser := ua.Parse(g.UserAgent)
		metrics.DeviceBreakdown[string(device)] += int(g.Count)
		metrics.BrowserBreakdown[string(browser)] += int(g.Count)
	}

	return metrics, nil
}
