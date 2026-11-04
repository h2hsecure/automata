package tracking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/domain/tracking"
)

// batchFakeRepo implements tracking.Repository, recording created events.
type batchFakeRepo struct {
	created int
}

func (r *batchFakeRepo) CreateEvent(e *tracking.Event) error {
	r.created++
	return nil
}
func (r *batchFakeRepo) CreateEventsBatch(events []*tracking.Event) error {
	r.created += len(events)
	return nil
}
func (r *batchFakeRepo) GetVisitor(contextID, visitorID string) (*tracking.Visitor, error) {
	return nil, nil
}
func (r *batchFakeRepo) UpsertVisitor(v *tracking.Visitor) error { return nil }
func (r *batchFakeRepo) GetDashboardMetrics(contextID string, dateRange tracking.DateRange) (*tracking.DashboardMetrics, error) {
	return nil, nil
}
func (r *batchFakeRepo) GetEvents(contextID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
	return nil, 0, nil
}
func (r *batchFakeRepo) GetTopPages(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.PageViewCount, error) {
	return nil, nil
}
func (r *batchFakeRepo) GetTopReferrers(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.ReferrerCount, error) {
	return nil, nil
}
func (r *batchFakeRepo) GetUAGroups(contextID string, dateRange tracking.DateRange) ([]tracking.UACount, error) {
	return nil, nil
}
func (r *batchFakeRepo) GetActiveVisitors(contextID string, minutes int) (int64, error) {
	return 0, nil
}
func (r *batchFakeRepo) PurgeOldEvents(contextID string, retentionDays int) (int64, error) {
	return 0, nil
}
func (r *batchFakeRepo) ListVisitors(ctx context.Context, contextID string, opts tracking.ListVisitorsOpts) ([]*tracking.Visitor, int64, error) {
	return nil, 0, nil
}
func (r *batchFakeRepo) GetVisitorByID(ctx context.Context, contextID, visitorID string) (*tracking.Visitor, error) {
	return nil, nil
}
func (r *batchFakeRepo) GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts tracking.GetVisitorEventsOpts) ([]*tracking.Event, int64, error) {
	return nil, 0, nil
}
func (r *batchFakeRepo) SetVisitorContactID(ctx context.Context, contextID, visitorID, contactID string) error {
	return nil
}
func (r *batchFakeRepo) ReassignVisitorContactID(contextID, oldContactID, newContactID string) error {
	return nil
}

// TestTrackBatch_CapsEventsPerRequest is the regression test for the body-size
// finding: a batch may carry at most 100 events, anything larger is rejected
// with 422 before any event is persisted.
func TestTrackBatch_CapsEventsPerRequest(t *testing.T) {
	t.Run("101 events rejected with 422", func(t *testing.T) {
		repo := &batchFakeRepo{}
		r := NewServer(nil, repo, nil)

		cid := uuid.New().String()
		var sb strings.Builder
		sb.WriteString(`{"events":[`)
		for i := 0; i < 101; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`{"contextId":"%s","type":"pageview"}`, cid))
		}
		sb.WriteString("]}")

		req := httptest.NewRequest("POST", "/track/batch", strings.NewReader(sb.String()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
		}
		if repo.created != 0 {
			t.Fatalf("expected 0 persisted events, got %d", repo.created)
		}
	})

	t.Run("100 events accepted", func(t *testing.T) {
		repo := &batchFakeRepo{}
		r := NewServer(nil, repo, nil)

		cid := uuid.New().String()
		var sb strings.Builder
		sb.WriteString(`{"events":[`)
		for i := 0; i < 100; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`{"contextId":"%s","type":"pageview"}`, cid))
		}
		sb.WriteString("]}")

		req := httptest.NewRequest("POST", "/track/batch", strings.NewReader(sb.String()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse body: %v", err)
		}
		if accepted, _ := body["accepted"].(float64); accepted != 100 {
			t.Fatalf("expected 100 accepted, got %v", body["accepted"])
		}
	})
}