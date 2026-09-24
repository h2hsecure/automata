package tracking

import "time"

type Visitor struct {
	ID          string
	ContextID   string
	CookieValue string
	Fingerprint string
	FirstSeen   time.Time
	LastSeen    time.Time
	PageViews   int
}
