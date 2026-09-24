package contact

import "time"

// ActivityType represents the type of contact activity.
type ActivityType string

const (
	ActivityFormSubmission ActivityType = "form_submission"
	ActivityPageVisit      ActivityType = "page_visit"
	ActivityBannerClick    ActivityType = "banner_click"
	ActivityContactCreated ActivityType = "contact_created"
	ActivityContactUpdated ActivityType = "contact_updated"
	ActivityContactMerged  ActivityType = "contact_merged"
)

// Activity represents a contact activity event.
type Activity struct {
	ID        string
	ContactID string
	ContextID  string
	Type      ActivityType
	Data      map[string]interface{}
	SourceID  string
	CreatedAt time.Time
}
