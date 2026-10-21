package tracking

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ct "github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/tracking"
	trackingUsecase "github.com/hekemen/automata/internal/usecase/tracking"
	"github.com/rs/zerolog/log"
)

// TrackHandler handles event tracking endpoints.
type TrackHandler struct {
	repo        Repository
	contactRepo ContactRepository
}

// Repository is the interface for tracking event persistence.
type Repository interface {
	tracking.Repository
}

// ContactRepository is the interface for contact persistence used by tracking
// (hidden visitor contacts + page_visit activities).
type ContactRepository interface {
	ct.Repository
}

// strField returns the first non-empty string value among the given keys.
func strField(raw map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := raw[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// eventFromRaw builds a tracking.Event from a raw JSON body. It accepts both
// the compact keys (v, t, u, r, p) and the legacy long keys (visitor_id,
// type, url, referrer, title, event_name, properties). The title falls back
// to p.title / p.pageTitle so nested titles are not lost, and the event name
// falls back to p.n for compact custom events.
func eventFromRaw(contextID string, raw map[string]interface{}, c *gin.Context) *tracking.Event {
	props, _ := raw["p"].(map[string]interface{})
	if props == nil {
		props, _ = raw["properties"].(map[string]interface{})
	}

	event := &tracking.Event{
		ID:         uuid.New().String(),
		ContextID:  contextID,
		VisitorID:  strField(raw, "v", "visitor_id"),
		Type:       tracking.EventType(strField(raw, "t", "type")),
		URL:        strField(raw, "u", "url"),
		Referrer:   strField(raw, "r", "referrer"),
		Title:      strField(raw, "title"),
		EventName:  strField(raw, "event_name"),
		Properties: props,
		UserAgent:  c.GetHeader("User-Agent"),
		IP:         c.ClientIP(),
		CreatedAt:  time.Now(),
	}

	if event.Title == "" && props != nil {
		if v, ok := props["title"].(string); ok {
			event.Title = v
		} else if v, ok := props["pageTitle"].(string); ok {
			event.Title = v
		}
	}
	if event.EventName == "" && props != nil {
		if v, ok := props["n"].(string); ok {
			event.EventName = v
		}
	}

	return event
}

// TrackScoped handles POST /track/:cid — compact event tracking with the
// context ID in the path (no contextId repeated in the body). The body uses
// short keys ({"v","t","u","r","p"}); legacy long keys are also accepted.
// The body may be sent with any Content-Type (the snippet uses text/plain to
// avoid a CORS preflight).
func (h *TrackHandler) TrackScoped(c *gin.Context) {
	contextID := c.Param("cid")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}

	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := eventFromRaw(contextID, raw, c)
	if event.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type required"})
		return
	}

	if err := trackingUsecase.TrackEvent(h.contactRepo, h.repo, contextID, event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// Track handles POST /track — single event tracking (legacy endpoint, kept
// for already-deployed snippets). The body uses long keys (contextId,
// visitor_id, type, url, referrer, title, event_name, properties); compact
// keys are also accepted.
func (h *TrackHandler) Track(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Prefer a valid contextId in the body (sent by the snippet); otherwise
	// fall back to header/subdomain resolution.
	contextID := ""
	if s := strField(raw, "contextId", "c"); s != "" {
		if _, err := uuid.Parse(s); err == nil {
			contextID = s
		}
	}
	if contextID == "" {
		contextID = ResolveContext(c)
		if contextID == "" {
			return
		}
	}

	event := eventFromRaw(contextID, raw, c)

	if err := trackingUsecase.TrackEvent(h.contactRepo, h.repo, contextID, event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// TrackBatch handles POST /track/batch — batch event tracking. Each event is
// routed through TrackEvent, so pageviews also produce hidden visitor
// contacts and page_visit activities. Events with an empty type (after the
// page_view→pageview normalization) are rejected.
func (h *TrackHandler) TrackBatch(c *gin.Context) {
	var req struct {
		Events []map[string]interface{} `json:"events"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Events may carry their own valid contextId; header/subdomain resolution
	// is used as a fallback only when needed.
	var fallbackContextID string
	fallbackResolved := false

	accepted, rejected := 0, 0
	for _, e := range req.Events {
		contextID := ""
		if v, ok := e["contextId"].(string); ok {
			if _, err := uuid.Parse(v); err == nil {
				contextID = v
			}
		}
		if contextID == "" {
			if !fallbackResolved {
				fallbackContextID = ResolveContext(c)
				fallbackResolved = true
				if fallbackContextID == "" {
					return
				}
			}
			contextID = fallbackContextID
		}

		event := &tracking.Event{
			ID:        uuid.New().String(),
			ContextID:  contextID,
			CreatedAt: time.Now(),
		}

		if v, ok := e["type"].(string); ok {
			event.Type = tracking.EventType(v)
		}
		if v, ok := e["url"].(string); ok {
			event.URL = v
		}
		if v, ok := e["title"].(string); ok {
			event.Title = v
		}
		if v, ok := e["referrer"].(string); ok {
			event.Referrer = v
		}
		if v, ok := e["event_name"].(string); ok {
			event.EventName = v
		}
		if v, ok := e["visitor_id"].(string); ok {
			event.VisitorID = v
		}
		if v, ok := e["properties"].(map[string]interface{}); ok {
			event.Properties = v
		}

		if event.Type == "" {
			rejected++
			continue
		}

		if err := trackingUsecase.TrackEvent(h.contactRepo, h.repo, contextID, event); err != nil {
			log.Warn().Err(err).Str("context_id", contextID).Msg("track batch: event failed")
			rejected++
			continue
		}
		accepted++
	}

	c.JSON(http.StatusOK, gin.H{"accepted": accepted, "rejected": rejected})
}
