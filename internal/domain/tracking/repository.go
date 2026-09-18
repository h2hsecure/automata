package tracking

import "time"

type DateRange struct {
	Start time.Time
	End   time.Time
}

type EventFilter struct {
	Type      EventType
	EventName string
	URL       string
	Offset    int
	Limit     int
}

type Repository interface {
	CreateEvent(e *Event) error
	CreateEventsBatch(events []*Event) error
	GetVisitor(tenantID, visitorID string) (*Visitor, error)
	UpsertVisitor(v *Visitor) error
	GetDashboardMetrics(tenantID string, dateRange DateRange) (*DashboardMetrics, error)
	GetEvents(tenantID string, opts EventFilter) ([]*Event, int64, error)
	GetTopPages(tenantID string, dateRange DateRange, limit int) ([]PageViewCount, error)
	GetTopReferrers(tenantID string, dateRange DateRange, limit int) ([]ReferrerCount, error)
	GetDeviceBreakdown(tenantID string, dateRange DateRange) (map[string]int, error)
	GetBrowserBreakdown(tenantID string, dateRange DateRange) (map[string]int, error)
	GetActiveVisitors(tenantID string, minutes int) (int64, error)
	PurgeOldEvents(tenantID string, retentionDays int) (int64, error)
}
