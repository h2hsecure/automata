package tracking

import (
	"context"
	"time"
)

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

	// ListVisitors returns a paginated list of visitors for a context.
	ListVisitors(ctx context.Context, contextID string, opts ListVisitorsOpts) ([]*Visitor, int64, error)

	// GetVisitorByID returns a single visitor by their ID (not cookie value).
	GetVisitorByID(ctx context.Context, contextID, visitorID string) (*Visitor, error)

	// GetVisitorEvents returns a paginated list of events for a specific visitor.
	GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts GetVisitorEventsOpts) ([]*Event, int64, error)

	// SetVisitorContactID links a visitor to a contact. It only sets the link
	// when the visitor is not already identified, so an upsert can never
	// un-identify a visitor.
	SetVisitorContactID(ctx context.Context, contextID, visitorID, contactID string) error
}
