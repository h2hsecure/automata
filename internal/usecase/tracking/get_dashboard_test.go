package tracking

import (
	"context"
	"testing"
	"time"

	"github.com/h2hsecure/automata/internal/domain/tracking"
)

type fakeDashRepo struct {
	metrics   *tracking.DashboardMetrics
	topPages  []tracking.PageViewCount
	topRefs   []tracking.ReferrerCount
	uaGroups  []tracking.UACount
	activeVis int64
	events    []*tracking.Event
	visitors  []*tracking.Visitor
}

func (f *fakeDashRepo) CreateEvent(e *tracking.Event) error {
	f.events = append(f.events, e)
	return nil
}

func (f *fakeDashRepo) CreateEventsBatch(events []*tracking.Event) error { return nil }

func (f *fakeDashRepo) GetVisitor(contextID, visitorID string) (*tracking.Visitor, error) {
	return nil, nil
}

func (f *fakeDashRepo) UpsertVisitor(v *tracking.Visitor) error {
	f.visitors = append(f.visitors, v)
	return nil
}

func (f *fakeDashRepo) GetDashboardMetrics(contextID string, dateRange tracking.DateRange) (*tracking.DashboardMetrics, error) {
	return f.metrics, nil
}

func (f *fakeDashRepo) GetEvents(contextID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
	return nil, 0, nil
}

func (f *fakeDashRepo) GetTopPages(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.PageViewCount, error) {
	return f.topPages, nil
}

func (f *fakeDashRepo) GetTopReferrers(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.ReferrerCount, error) {
	return f.topRefs, nil
}

func (f *fakeDashRepo) GetUAGroups(contextID string, dateRange tracking.DateRange) ([]tracking.UACount, error) {
	return f.uaGroups, nil
}

func (f *fakeDashRepo) GetActiveVisitors(contextID string, minutes int) (int64, error) {
	return f.activeVis, nil
}

func (f *fakeDashRepo) PurgeOldEvents(contextID string, retentionDays int) (int64, error) {
	return 0, nil
}

func (f *fakeDashRepo) ListVisitors(ctx context.Context, contextID string, opts tracking.ListVisitorsOpts) ([]*tracking.Visitor, int64, error) {
	return nil, 0, nil
}

func (f *fakeDashRepo) GetVisitorByID(ctx context.Context, contextID, visitorID string) (*tracking.Visitor, error) {
	return nil, nil
}

func (f *fakeDashRepo) GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts tracking.GetVisitorEventsOpts) ([]*tracking.Event, int64, error) {
	return nil, 0, nil
}

func (f *fakeDashRepo) SetVisitorContactID(ctx context.Context, contextID, visitorID, contactID string) error {
	return nil
}

func (f *fakeDashRepo) ReassignVisitorContactID(contextID, oldContactID, newContactID string) error {
	return nil
}

func TestGetDashboard_MergesMetricsAndBucketsUA(t *testing.T) {
	repo := &fakeDashRepo{
		metrics:   &tracking.DashboardMetrics{TotalVisitors: 5, PageViews: 10},
		topPages:  []tracking.PageViewCount{{URL: "/home", Count: 7}},
		topRefs:   []tracking.ReferrerCount{{Referrer: "google.com", Count: 3}},
		uaGroups: []tracking.UACount{
			{UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", Count: 3},
			{UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", Count: 2},
			{UserAgent: "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", Count: 1},
		},
		activeVis: 2,
	}

	metrics, err := GetDashboard(repo, "ctx-1", GetDashboardOptions{
		Start: time.Now().AddDate(0, 0, -30),
		End:   time.Now(),
	})
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	// Metrics from GetDashboardMetrics must be merged, not overwrite the
	// already-populated TopPages/TopReferrers.
	if metrics.TotalVisitors != 5 || metrics.PageViews != 10 {
		t.Errorf("counts = (%d, %d), want (5, 10)", metrics.TotalVisitors, metrics.PageViews)
	}
	if metrics.ActiveVisitors != 2 {
		t.Errorf("ActiveVisitors = %d, want 2", metrics.ActiveVisitors)
	}
	if len(metrics.TopPages) != 1 || metrics.TopPages[0].URL != "/home" {
		t.Errorf("TopPages = %+v, want [/home]", metrics.TopPages)
	}
	if len(metrics.TopReferrers) != 1 || metrics.TopReferrers[0].Referrer != "google.com" {
		t.Errorf("TopReferrers = %+v, want [google.com]", metrics.TopReferrers)
	}

	wantDevice := map[string]int{"desktop": 3, "mobile": 3}
	wantBrowser := map[string]int{"Chrome": 4, "Safari": 2}
	if !mapsEqual(metrics.DeviceBreakdown, wantDevice) {
		t.Errorf("DeviceBreakdown = %v, want %v", metrics.DeviceBreakdown, wantDevice)
	}
	if !mapsEqual(metrics.BrowserBreakdown, wantBrowser) {
		t.Errorf("BrowserBreakdown = %v, want %v", metrics.BrowserBreakdown, wantBrowser)
	}
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}