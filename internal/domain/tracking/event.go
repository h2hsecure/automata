package tracking

import "time"

type EventType string

const (
	EventTypePageView EventType = "pageview"
	EventTypeEvent    EventType = "event"
)

type Event struct {
	ID          string
	TenantID    string
	VisitorID   string
	Type        EventType
	URL         string
	Title       string
	Referrer    string
	EventName   string
	Properties  map[string]interface{}
	UserAgent   string
	IPHash      string
	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	CreatedAt   time.Time
}

func (e *Event) IsPageView() bool {
	return e.Type == EventTypePageView
}
