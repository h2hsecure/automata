package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/tracking"
)

// fakeEventsRepo embeds tracking.Repository so only GetEvents must be
// implemented. It records the filter it receives so tests can assert the
// handler clamped limit/offset before the query ran.
type fakeEventsRepo struct {
	tracking.Repository
	seed     []*tracking.Event
	lastOpts tracking.EventFilter
}

func (f *fakeEventsRepo) GetEvents(contextID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
	f.lastOpts = opts
	start := opts.Offset
	if start < 0 {
		start = 0
	}
	if start > len(f.seed) {
		start = len(f.seed)
	}
	end := len(f.seed)
	if opts.Limit > 0 {
		if e := start + opts.Limit; e < end {
			end = e
		}
	}
	return f.seed[start:end], int64(len(f.seed)), nil
}

func seedEvents(n int) []*tracking.Event {
	events := make([]*tracking.Event, n)
	for i := range events {
		events[i] = &tracking.Event{ID: fmt.Sprintf("evt-%d", i), ContextID: "ctx-1"}
	}
	return events
}

func newEventsTestContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", path, bytes.NewReader(nil))
	c.Set("context_id", "ctx-1")
	return c, w
}

func TestGetEvents_NoLimitDefaultsTo50(t *testing.T) {
	repo := &fakeEventsRepo{seed: seedEvents(60)}
	h := NewTrackingHandler(repo)

	c, w := newEventsTestContext("/context/tracking/events")
	h.GetEvents(c)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Handler must default the limit to 50 so the repo never runs unbounded.
	if repo.lastOpts.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", repo.lastOpts.Limit)
	}
	if repo.lastOpts.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", repo.lastOpts.Offset)
	}

	var resp struct {
		Events []*tracking.Event `json:"events"`
		Total  int64             `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Events) != 50 {
		t.Errorf("expected 50 events returned, got %d", len(resp.Events))
	}
}

func TestGetEvents_LimitClampedTo100(t *testing.T) {
	repo := &fakeEventsRepo{seed: seedEvents(150)}
	h := NewTrackingHandler(repo)

	c, w := newEventsTestContext("/context/tracking/events?limit=1000")
	h.GetEvents(c)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// limit=1000 must be clamped to the max of 100.
	if repo.lastOpts.Limit != 100 {
		t.Errorf("expected clamped limit 100, got %d", repo.lastOpts.Limit)
	}
}

func TestGetEvents_NegativeLimitTreatedAsDefault(t *testing.T) {
	repo := &fakeEventsRepo{seed: seedEvents(60)}
	h := NewTrackingHandler(repo)

	c, w := newEventsTestContext("/context/tracking/events?limit=-5")
	h.GetEvents(c)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.lastOpts.Limit != 50 {
		t.Errorf("expected negative limit to fall back to default 50, got %d", repo.lastOpts.Limit)
	}
}