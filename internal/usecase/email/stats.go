package email

import (
	"fmt"
	"time"

	cd "github.com/hekemen/automata/internal/domain/contact"
)

// EmailStats aggregates dashboard metrics for a context's email activity.
type EmailStats struct {
	Pending         int `json:"pending"`
	Sent24h         int `json:"sent_24h"`
	Failed24h       int `json:"failed_24h"`
	EmailsSentTotal int `json:"emails_sent_total"`
	ContactsReached int `json:"contacts_reached"`
	Reached7d       int `json:"reached_7d"`
}

// GetEmailStats returns dashboard stats for a context: queue depth, 24h
// send/fail counts, total sends, and distinct contacts reached (all time and
// in the last 7 days). Contact counts are 0 when no contact repository is
// wired in.
func (u *EmailUsecase) GetEmailStats(contextID string) (*EmailStats, error) {
	stats := &EmailStats{}

	pending, err := u.queue.CountPending(contextID)
	if err != nil {
		return nil, fmt.Errorf("count pending: %w", err)
	}
	stats.Pending = pending

	now := time.Now()
	since24h := now.Add(-24 * time.Hour)
	since7d := now.Add(-7 * 24 * time.Hour)

	sent24h, err := u.queue.CountHistory(contextID, "sent", &since24h)
	if err != nil {
		return nil, fmt.Errorf("count sent 24h: %w", err)
	}
	stats.Sent24h = sent24h

	failed24h, err := u.queue.CountHistory(contextID, "failed", &since24h)
	if err != nil {
		return nil, fmt.Errorf("count failed 24h: %w", err)
	}
	stats.Failed24h = failed24h

	sentTotal, err := u.queue.CountHistory(contextID, "sent", nil)
	if err != nil {
		return nil, fmt.Errorf("count sent total: %w", err)
	}
	stats.EmailsSentTotal = sentTotal

	if u.contactRepo != nil {
		reached, err := u.contactRepo.CountDistinctContactsByActivity(contextID, string(cd.ActivityEmailSent), nil)
		if err != nil {
			return nil, fmt.Errorf("count contacts reached: %w", err)
		}
		stats.ContactsReached = reached

		reached7d, err := u.contactRepo.CountDistinctContactsByActivity(contextID, string(cd.ActivityEmailSent), &since7d)
		if err != nil {
			return nil, fmt.Errorf("count contacts reached 7d: %w", err)
		}
		stats.Reached7d = reached7d
	}

	return stats, nil
}
