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
	GetVisitor(contextID, visitorID string) (*Visitor, error)
	UpsertVisitor(v *Visitor) error
	GetDashboardMetrics(contextID string, dateRange DateRange) (*DashboardMetrics, error)
	GetEvents(contextID string, opts EventFilter) ([]*Event, int64, error)
	GetTopPages(contextID string, dateRange DateRange, limit int) ([]PageViewCount, error)
	GetTopReferrers(contextID string, dateRange DateRange, limit int) ([]ReferrerCount, error)
	GetDeviceBreakdown(contextID string, dateRange DateRange) (map[string]int, error)
	GetBrowserBreakdown(contextID string, dateRange DateRange) (map[string]int, error)
	GetActiveVisitors(contextID string, minutes int) (int64, error)
	PurgeOldEvents(contextID string, retentionDays int) (int64, error)
}
