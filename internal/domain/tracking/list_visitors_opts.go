package tracking

// ListVisitorsOpts holds options for listing visitors.
type ListVisitorsOpts struct {
	Offset int
	Limit  int
	Sort   string // "first_seen", "last_seen", "page_views"
}

// GetVisitorEventsOpts holds options for listing a visitor's events.
type GetVisitorEventsOpts struct {
	Offset int
	Limit  int
	Type   EventType
}
