package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/tracking"
	trackingUsecase "github.com/hekemen/automata/internal/usecase/tracking"
)

// TrackingHandler handles dashboard and events API endpoints.
type TrackingHandler struct {
	repo tracking.Repository
}

// NewTrackingHandler creates a new TrackingHandler.
func NewTrackingHandler(repo tracking.Repository) *TrackingHandler {
	return &TrackingHandler{repo: repo}
}

// GetDashboard handles GET /api/tracking/dashboard — returns dashboard metrics.
func (h *TrackingHandler) GetDashboard(c *gin.Context) {
	context := c.GetString("context")
	if context == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	var start, end time.Time

	if s := c.Query("start"); s != "" {
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			start = parsed
		}
	}
	if e := c.Query("end"); e != "" {
		if parsed, err := time.Parse(time.RFC3339, e); err == nil {
			end = parsed
		}
	}

	opts := trackingUsecase.GetDashboardOptions{
		Start:     start,
		End:       end,
		ActiveMin: 30,
		TopN:      10,
	}

	metrics, err := trackingUsecase.GetDashboard(h.repo, context, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_visitors":   metrics.TotalVisitors,
		"active_visitors":  metrics.ActiveVisitors,
		"page_views":       metrics.PageViews,
		"top_pages":        metrics.TopPages,
		"top_referrers":    metrics.TopReferrers,
		"device_breakdown": metrics.DeviceBreakdown,
		"browser_breakdown": metrics.BrowserBreakdown,
	})
}

// GetEvents handles GET /api/tracking/events — returns paginated events.
func (h *TrackingHandler) GetEvents(c *gin.Context) {
	context := c.GetString("context")
	if context == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	var opts tracking.EventFilter

	if t := c.Query("type"); t != "" {
		opts.Type = tracking.EventType(t)
	}
	if en := c.Query("event_name"); en != "" {
		opts.EventName = en
	}
	if u := c.Query("url"); u != "" {
		opts.URL = u
	}

	if o := c.Query("offset"); o != "" {
		if p, err := strconv.Atoi(o); err == nil {
			opts.Offset = p
		}
	}
	if l := c.Query("limit"); l != "" {
		if p, err := strconv.Atoi(l); err == nil {
			opts.Limit = p
		}
	}

	events, total, err := trackingUsecase.GetEvents(h.repo, context, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  total,
	})
}
