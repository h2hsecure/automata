package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "embed"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/tracking"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

// RunMigrations executes the embedded migration.sql against the provided database pool.
func RunMigrations(db *pgxpool.Pool) error {
	if _, err := db.Exec(context.Background(), migrationSQL); err != nil {
		return fmt.Errorf("execute tracking migration: %w", err)
	}
	return nil
}

// repo implements the tracking.Repository interface using PostgreSQL.
type repo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL tracking repository.
func New(pool *pgxpool.Pool) tracking.Repository {
	return &repo{pool: pool}
}

func (r *repo) CreateEvent(e *tracking.Event) error {
	ctx := context.Background()
	propertiesJSON, err := json.Marshal(e.Properties)
	if err != nil {
		return fmt.Errorf("marshal event properties: %w", err)
	}
	query := `
		INSERT INTO tracking_events (id, context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
	`
	_, err = r.pool.Exec(ctx, query, e.ID, e.ContextID, e.VisitorID, e.Type, e.URL, e.Title, e.Referrer, e.EventName, propertiesJSON, e.UserAgent, e.IPHash, e.UTMSource, e.UTMMedium, e.UTMCampaign)
	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}

func (r *repo) CreateEventsBatch(events []*tracking.Event) error {
	ctx := context.Background()
	batch := &pgx.Batch{}
	for _, e := range events {
		propertiesJSON, err := json.Marshal(e.Properties)
		if err != nil {
			return fmt.Errorf("marshal event properties: %w", err)
		}
		query := `
			INSERT INTO tracking_events (id, context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		`
		batch.Queue(query, e.ID, e.ContextID, e.VisitorID, e.Type, e.URL, e.Title, e.Referrer, e.EventName, propertiesJSON, e.UserAgent, e.IPHash, e.UTMSource, e.UTMMedium, e.UTMCampaign)
	}
	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()
	for i := range events {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("batch create events (query %d): %w", i, err)
		}
	}
	return nil
}

func (r *repo) GetVisitor(contextID, visitorID string) (*tracking.Visitor, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views
		FROM tracking_visitors
		WHERE context_id = $1 AND cookie_value = $2
	`
	v := &tracking.Visitor{}
	err := r.pool.QueryRow(ctx, query, contextID, visitorID).Scan(
		&v.ID, &v.ContextID, &v.CookieValue, &v.Fingerprint,
		&v.FirstSeen, &v.LastSeen, &v.PageViews,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("visitor not found: %w", err)
		}
		return nil, fmt.Errorf("get visitor: %w", err)
	}
	return v, nil
}

func (r *repo) UpsertVisitor(v *tracking.Visitor) error {
	ctx := context.Background()
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	query := `
		INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (context_id, cookie_value) DO UPDATE
			SET last_seen = EXCLUDED.last_seen,
				page_views = tracking_visitors.page_views + EXCLUDED.page_views
	`
	_, err := r.pool.Exec(ctx, query, v.ID, v.ContextID, v.CookieValue, v.Fingerprint, v.FirstSeen, v.LastSeen, v.PageViews)
	if err != nil {
		return fmt.Errorf("upsert visitor: %w", err)
	}
	return nil
}

func (r *repo) GetDashboardMetrics(contextID string, dateRange tracking.DateRange) (*tracking.DashboardMetrics, error) {
	ctx := context.Background()
	metrics := &tracking.DashboardMetrics{}

	var totalVisitors, pageViews int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT visitor_id), COUNT(*)
		FROM tracking_events
		WHERE context_id = $1 AND created_at BETWEEN $2 AND $3
	`, contextID, dateRange.Start, dateRange.End).Scan(&totalVisitors, &pageViews)
	if err != nil {
		return nil, fmt.Errorf("get dashboard counts: %w", err)
	}
	metrics.TotalVisitors = totalVisitors
	metrics.PageViews = pageViews

	return metrics, nil
}

func (r *repo) GetEvents(contextID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
	ctx := context.Background()

	countQuery := `SELECT COUNT(*) FROM tracking_events WHERE context_id = $1`
	dataQuery := `
		SELECT id, context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at
		FROM tracking_events WHERE context_id = $1
	`
	args := []interface{}{contextID}
	argIdx := 2

	if opts.Type != "" {
		countQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		dataQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, opts.Type)
		argIdx++
	}
	if opts.EventName != "" {
		countQuery += fmt.Sprintf(" AND event_name = $%d", argIdx)
		dataQuery += fmt.Sprintf(" AND event_name = $%d", argIdx)
		args = append(args, opts.EventName)
		argIdx++
	}
	if opts.URL != "" {
		countQuery += fmt.Sprintf(" AND url = $%d", argIdx)
		dataQuery += fmt.Sprintf(" AND url = $%d", argIdx)
		args = append(args, opts.URL)
		argIdx++
	}

	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}

	dataQuery += " ORDER BY created_at DESC"
	if opts.Limit > 0 {
		dataQuery += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			dataQuery += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []*tracking.Event
	for rows.Next() {
		e := &tracking.Event{}
		var propertiesJSON []byte
		err := rows.Scan(
			&e.ID, &e.ContextID, &e.VisitorID, &e.Type, &e.URL, &e.Title,
			&e.Referrer, &e.EventName, &propertiesJSON, &e.UserAgent, &e.IPHash,
			&e.UTMSource, &e.UTMMedium, &e.UTMCampaign, &e.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan event: %w", err)
		}
		if err := json.Unmarshal(propertiesJSON, &e.Properties); err != nil {
			return nil, 0, fmt.Errorf("unmarshal event properties: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}
	return events, total, nil
}

func (r *repo) GetTopPages(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.PageViewCount, error) {
	ctx := context.Background()
	query := `
		SELECT url, COUNT(*) as count
		FROM tracking_events
		WHERE context_id = $1 AND type = 'pageview' AND created_at BETWEEN $2 AND $3
		GROUP BY url ORDER BY count DESC LIMIT $4
	`
	rows, err := r.pool.Query(ctx, query, contextID, dateRange.Start, dateRange.End, limit)
	if err != nil {
		return nil, fmt.Errorf("get top pages: %w", err)
	}
	defer rows.Close()

	var pages []tracking.PageViewCount
	for rows.Next() {
		var p tracking.PageViewCount
		err := rows.Scan(&p.URL, &p.Count)
		if err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		pages = append(pages, p)
	}
	return pages, nil
}

func (r *repo) GetTopReferrers(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.ReferrerCount, error) {
	ctx := context.Background()
	query := `
		SELECT referrer, COUNT(*) as count
		FROM tracking_events
		WHERE context_id = $1 AND referrer != '' AND created_at BETWEEN $2 AND $3
		GROUP BY referrer ORDER BY count DESC LIMIT $4
	`
	rows, err := r.pool.Query(ctx, query, contextID, dateRange.Start, dateRange.End, limit)
	if err != nil {
		return nil, fmt.Errorf("get top referrers: %w", err)
	}
	defer rows.Close()

	var referrers []tracking.ReferrerCount
	for rows.Next() {
		var r tracking.ReferrerCount
		err := rows.Scan(&r.Referrer, &r.Count)
		if err != nil {
			return nil, fmt.Errorf("scan referrer: %w", err)
		}
		referrers = append(referrers, r)
	}
	return referrers, nil
}

func (r *repo) GetDeviceBreakdown(contextID string, dateRange tracking.DateRange) (map[string]int, error) {
	ctx := context.Background()
	query := `
		SELECT user_agent, COUNT(*) as count
		FROM tracking_events
		WHERE context_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY user_agent ORDER BY count DESC
	`
	rows, err := r.pool.Query(ctx, query, contextID, dateRange.Start, dateRange.End)
	if err != nil {
		return nil, fmt.Errorf("get device breakdown: %w", err)
	}
	defer rows.Close()

	breakdown := make(map[string]int)
	for rows.Next() {
		var ua string
		var count int
		err := rows.Scan(&ua, &count)
		if err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		breakdown[ua] = count
	}
	return breakdown, nil
}

func (r *repo) GetBrowserBreakdown(contextID string, dateRange tracking.DateRange) (map[string]int, error) {
	ctx := context.Background()
	query := `
		SELECT user_agent, COUNT(*) as count
		FROM tracking_events
		WHERE context_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY user_agent ORDER BY count DESC
	`
	rows, err := r.pool.Query(ctx, query, contextID, dateRange.Start, dateRange.End)
	if err != nil {
		return nil, fmt.Errorf("get browser breakdown: %w", err)
	}
	defer rows.Close()

	breakdown := make(map[string]int)
	for rows.Next() {
		var ua string
		var count int
		err := rows.Scan(&ua, &count)
		if err != nil {
			return nil, fmt.Errorf("scan browser: %w", err)
		}
		breakdown[ua] = count
	}
	return breakdown, nil
}

func (r *repo) GetActiveVisitors(contextID string, minutes int) (int64, error) {
	ctx := context.Background()
	query := `
		SELECT COUNT(DISTINCT visitor_id)
		FROM tracking_events
		WHERE context_id = $1 AND created_at > NOW() - ($2 || ' minutes')::interval
	`
	var count int64
	err := r.pool.QueryRow(ctx, query, contextID, fmt.Sprintf("%d", minutes)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get active visitors: %w", err)
	}
	return count, nil
}

func (r *repo) PurgeOldEvents(contextID string, retentionDays int) (int64, error) {
	ctx := context.Background()
	query := `
		DELETE FROM tracking_events
		WHERE context_id = $1 AND created_at < NOW() - ($2 || ' days')::interval
	`
	result, err := r.pool.Exec(ctx, query, contextID, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("purge old events: %w", err)
	}
	return result.RowsAffected(), nil
}

func (r *repo) ListVisitors(ctx context.Context, contextID string, opts tracking.ListVisitorsOpts) ([]*tracking.Visitor, int64, error) {
	if opts.Sort == "" {
		opts.Sort = "last_seen"
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}

	// Build sort clause
	var sortCol string
	switch opts.Sort {
	case "first_seen":
		sortCol = "first_seen"
	case "last_seen":
		sortCol = "last_seen"
	case "page_views":
		sortCol = "page_views"
	default:
		return nil, 0, fmt.Errorf("invalid sort field: %s", opts.Sort)
	}

	countQuery := `SELECT COUNT(*) FROM tracking_visitors WHERE context_id = $1`
	dataQuery := `
		SELECT id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views
		FROM tracking_visitors
		WHERE context_id = $1
		ORDER BY ` + sortCol + ` DESC
	`
	args := []interface{}{contextID}
	argIdx := 2

	dataQuery += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, opts.Limit)
	argIdx++
	if opts.Offset > 0 {
		dataQuery += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, opts.Offset)
		argIdx++
	}

	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, contextID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count visitors: %w", err)
	}

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query visitors: %w", err)
	}
	defer rows.Close()

	var visitors []*tracking.Visitor
	for rows.Next() {
		v := &tracking.Visitor{}
		err := rows.Scan(&v.ID, &v.ContextID, &v.CookieValue, &v.Fingerprint, &v.FirstSeen, &v.LastSeen, &v.PageViews)
		if err != nil {
			return nil, 0, fmt.Errorf("scan visitor: %w", err)
		}
		visitors = append(visitors, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate visitors: %w", err)
	}
	return visitors, total, nil
}

func (r *repo) GetVisitorByID(ctx context.Context, contextID, visitorID string) (*tracking.Visitor, error) {
	query := `
		SELECT id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views
		FROM tracking_visitors
		WHERE context_id = $1 AND id = $2
	`
	v := &tracking.Visitor{}
	err := r.pool.QueryRow(ctx, query, contextID, visitorID).Scan(
		&v.ID, &v.ContextID, &v.CookieValue, &v.Fingerprint,
		&v.FirstSeen, &v.LastSeen, &v.PageViews,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("visitor not found")
		}
		return nil, fmt.Errorf("get visitor by id: %w", err)
	}
	return v, nil
}

func (r *repo) GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts tracking.GetVisitorEventsOpts) ([]*tracking.Event, int64, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}

	countQuery := `SELECT COUNT(*) FROM tracking_events WHERE context_id = $1 AND visitor_id = $2`
	dataQuery := `
		SELECT id, context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at
		FROM tracking_events
		WHERE context_id = $1 AND visitor_id = $2
	`
	args := []interface{}{contextID, visitorID}
	argIdx := 3

	if opts.Type != "" {
		countQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		dataQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, opts.Type)
		argIdx++
	}

	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count visitor events: %w", err)
	}

	dataQuery += " ORDER BY created_at DESC"
	dataQuery += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, opts.Limit)
	argIdx++
	if opts.Offset > 0 {
		dataQuery += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, opts.Offset)
		argIdx++
	}

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query visitor events: %w", err)
	}
	defer rows.Close()

	var events []*tracking.Event
	for rows.Next() {
		e := &tracking.Event{}
		var propertiesJSON []byte
		err := rows.Scan(
			&e.ID, &e.ContextID, &e.VisitorID, &e.Type, &e.URL, &e.Title,
			&e.Referrer, &e.EventName, &propertiesJSON, &e.UserAgent, &e.IPHash,
			&e.UTMSource, &e.UTMMedium, &e.UTMCampaign, &e.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan event: %w", err)
		}
		if err := json.Unmarshal(propertiesJSON, &e.Properties); err != nil {
			return nil, 0, fmt.Errorf("unmarshal event properties: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}
	return events, total, nil
}
