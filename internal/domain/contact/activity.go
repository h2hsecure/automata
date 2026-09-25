package contact

import (
	"fmt"
	"time"
)

// ActivityType represents the type of contact activity.
type ActivityType string

const (
	ActivityFormSubmission   ActivityType = "form_submission"
	ActivityPageVisit        ActivityType = "page_visit"
	ActivityBannerClick      ActivityType = "banner_click"
	ActivityBannerImpression ActivityType = "banner_impression"
	ActivityContactCreated   ActivityType = "contact_created"
	ActivityContactUpdated   ActivityType = "contact_updated"
	ActivityContactMerged    ActivityType = "contact_merged"
	ActivityTagAdded         ActivityType = "tag_added"
	ActivityTagRemoved       ActivityType = "tag_removed"
)

// Activity represents a contact activity event.
type Activity struct {
	ID        string
	ContactID string
	ContextID string
	Type      ActivityType
	Data      map[string]interface{}
	SourceID  string
	CreatedAt time.Time
}

// ActivityDetail extends Activity with human-readable title and description.
type ActivityDetail struct {
	ID          string                 `json:"id"`
	Type        ActivityType           `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ActivityResult wraps paginated activities with a total count.
type ActivityResult struct {
	Activities []ActivityDetail
	Total      int64
}

// FormatActivityTitle generates a human-readable title for an activity.
func FormatActivityTitle(a Activity) string {
	switch a.Type {
	case ActivityFormSubmission:
		if fn, ok := a.Data["form_name"].(string); ok && fn != "" {
			return fmt.Sprintf("Submitted '%s' form", fn)
		}
		return "Form submission"
	case ActivityPageVisit:
		if url, ok := a.Data["url"].(string); ok && url != "" {
			return fmt.Sprintf("Visited %s", url)
		}
		return "Page visit"
	case ActivityBannerImpression:
		if bn, ok := a.Data["banner_name"].(string); ok && bn != "" {
			return fmt.Sprintf("Viewed banner '%s'", bn)
		}
		return "Banner impression"
	case ActivityBannerClick:
		if bn, ok := a.Data["banner_name"].(string); ok && bn != "" {
			return fmt.Sprintf("Clicked banner '%s'", bn)
		}
		return "Banner click"
	case ActivityContactCreated:
		return "Contact created"
	case ActivityContactUpdated:
		return "Contact updated"
	case ActivityContactMerged:
		return "Contact merged"
	case ActivityTagAdded:
		if tn, ok := a.Data["tag_name"].(string); ok && tn != "" {
			return fmt.Sprintf("Tag added: %s", tn)
		}
		return "Tag added"
	case ActivityTagRemoved:
		if tn, ok := a.Data["tag_name"].(string); ok && tn != "" {
			return fmt.Sprintf("Tag removed: %s", tn)
		}
		return "Tag removed"
	default:
		return string(a.Type)
	}
}

// FormatActivityDescription generates a brief description for an activity.
func FormatActivityDescription(a Activity) string {
	switch a.Type {
	case ActivityFormSubmission:
		if fn, ok := a.Data["form_name"].(string); ok && fn != "" {
			return fn
		}
		return "Form submission"
	case ActivityPageVisit:
		if title, ok := a.Data["title"].(string); ok && title != "" {
			return title
		}
		if url, ok := a.Data["url"].(string); ok && url != "" {
			return url
		}
		return "Page visit"
	case ActivityBannerImpression:
		if bn, ok := a.Data["banner_name"].(string); ok && bn != "" {
			return bn
		}
		return "Banner view"
	case ActivityBannerClick:
		if bn, ok := a.Data["banner_name"].(string); ok && bn != "" {
			return bn
		}
		return "Banner click"
	case ActivityContactCreated:
		return "New contact"
	case ActivityContactUpdated:
		return "Contact details modified"
	case ActivityContactMerged:
		return "Contact merged with another"
	case ActivityTagAdded:
		return "Tag assigned"
	case ActivityTagRemoved:
		return "Tag removed"
	default:
		return string(a.Type)
	}
}
