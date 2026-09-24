package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "embed"
	"time"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/banner"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

// RunMigrations executes the embedded migration.sql against the provided database pool.
func RunMigrations(db *pgxpool.Pool) error {
	if _, err := db.Exec(context.Background(), migrationSQL); err != nil {
		return fmt.Errorf("execute banner migration: %w", err)
	}
	return nil
}

// bannerRepo implements the banner.Repository interface using PostgreSQL.
type bannerRepo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL banner repository.
func New(pool *pgxpool.Pool) banner.Repository {
	return &bannerRepo{pool: pool}
}

// --- Placement operations ---

func (r *bannerRepo) CreatePlacement(p *banner.Placement) error {
	ctx := context.Background()
	p.ID = uuid.New().String()
	query := `
		INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query, p.ID, p.ContextID, p.Name, p.Location, p.CSSSelector, p.MaxBanners, p.Priority, p.IsActive)
	if err != nil {
		return fmt.Errorf("create placement: %w", err)
	}
	return nil
}

func (r *bannerRepo) GetPlacement(id string) (*banner.Placement, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at
		FROM banner_placements WHERE id = $1
	`
	p := &banner.Placement{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.ContextID, &p.Name, &p.Location, &p.CSSSelector, &p.MaxBanners, &p.Priority,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("placement not found")
		}
		return nil, fmt.Errorf("get placement: %w", err)
	}
	return p, nil
}

func (r *bannerRepo) ListPlacements(contextID string, isActive bool) ([]*banner.Placement, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at
		FROM banner_placements WHERE context_id = $1
	`
	args := []interface{}{contextID}
	argIdx := 2

	if isActive {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}
	query += " ORDER BY priority DESC, created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list placements: %w", err)
	}
	defer rows.Close()

	var placements []*banner.Placement
	for rows.Next() {
		p := &banner.Placement{}
		err := rows.Scan(&p.ID, &p.ContextID, &p.Name, &p.Location, &p.CSSSelector, &p.MaxBanners, &p.Priority, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan placement: %w", err)
		}
		placements = append(placements, p)
	}
	return placements, nil
}

func (r *bannerRepo) UpdatePlacement(p *banner.Placement) error {
	ctx := context.Background()
	query := `
		UPDATE banner_placements
		SET name = $1, location = $2, css_selector = $3, max_banners = $4, priority = $5, is_active = $6, updated_at = NOW()
		WHERE id = $7
	`
	_, err := r.pool.Exec(ctx, query, p.Name, p.Location, p.CSSSelector, p.MaxBanners, p.Priority, p.IsActive, p.ID)
	if err != nil {
		return fmt.Errorf("update placement: %w", err)
	}
	return nil
}

func (r *bannerRepo) DeletePlacement(id string) error {
	ctx := context.Background()
	query := `DELETE FROM banner_placements WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete placement: %w", err)
	}
	return nil
}

// --- Banner operations ---

func (r *bannerRepo) CreateBanner(b *banner.Banner) error {
	ctx := context.Background()
	b.ID = uuid.New().String()
	placementsJSON, err := json.Marshal(b.Placements)
	if err != nil {
		return fmt.Errorf("marshal banner placements: %w", err)
	}
	abVariantsJSON, err := json.Marshal(b.ABVariants)
	if err != nil {
		return fmt.Errorf("marshal banner ab_variants: %w", err)
	}
	query := `
		INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, 0, 0, NOW(), NOW())
	`
	_, err = r.pool.Exec(ctx, query, b.ID, b.ContextID, b.Name, b.Type, b.Content, b.LinkURL,
		b.ImageURL, b.AltText, b.CampaignID, placementsJSON, b.Priority, b.StartDate, b.EndDate,
		b.IsActive, b.ABTest, abVariantsJSON)
	if err != nil {
		return fmt.Errorf("create banner: %w", err)
	}
	return nil
}

func (r *bannerRepo) GetBanner(id string) (*banner.Banner, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at
		FROM banner_banners WHERE id = $1
	`
	b := &banner.Banner{}
	var placementsJSON, abVariantsJSON []byte
	var imageURL, altText, campaignID *string
	var startDate, endDate *time.Time
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.ContextID, &b.Name, &b.Type, &b.Content, &b.LinkURL, &imageURL, &altText,
		&campaignID, &placementsJSON, &b.Priority, &startDate, &endDate, &b.IsActive,
		&b.ABTest, &abVariantsJSON, &b.Impressions, &b.Clicks, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("banner not found")
		}
		return nil, fmt.Errorf("get banner: %w", err)
	}
	b.ImageURL = imageURL
	b.AltText = altText
	b.CampaignID = campaignID
	b.StartDate = startDate
	b.EndDate = endDate
	if err := json.Unmarshal(placementsJSON, &b.Placements); err != nil {
		return nil, fmt.Errorf("unmarshal banner placements: %w", err)
	}
	if err := json.Unmarshal(abVariantsJSON, &b.ABVariants); err != nil {
		return nil, fmt.Errorf("unmarshal banner ab_variants: %w", err)
	}
	return b, nil
}

func (r *bannerRepo) ListBanners(contextID string, opts banner.ListOptions) ([]*banner.Banner, int64, error) {
	ctx := context.Background()
	countQuery := `SELECT COUNT(*) FROM banner_banners WHERE context_id = $1`
	query := `
		SELECT id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at
		FROM banner_banners WHERE context_id = $1
	`
	args := []interface{}{contextID}
	argIdx := 2

	if opts.CampaignID != "" {
		countQuery += fmt.Sprintf(" AND campaign_id = $%d", argIdx)
		query += fmt.Sprintf(" AND campaign_id = $%d", argIdx)
		args = append(args, opts.CampaignID)
		argIdx++
	}
	if opts.IsActive {
		countQuery += fmt.Sprintf(" AND is_active = $%d", argIdx)
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}

	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count banners: %w", err)
	}

	query += " ORDER BY priority DESC, created_at DESC"
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, opts.Limit)
		argIdx++
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIdx)
			args = append(args, opts.Offset)
			argIdx++
		}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list banners: %w", err)
	}
	defer rows.Close()

	var banners []*banner.Banner
	for rows.Next() {
		b := &banner.Banner{}
		var placementsJSON, abVariantsJSON []byte
		var imageURL, altText, campaignID *string
		var startDate, endDate *time.Time
		err := rows.Scan(
			&b.ID, &b.ContextID, &b.Name, &b.Type, &b.Content, &b.LinkURL, &imageURL, &altText,
			&campaignID, &placementsJSON, &b.Priority, &startDate, &endDate, &b.IsActive,
			&b.ABTest, &abVariantsJSON, &b.Impressions, &b.Clicks, &b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan banner: %w", err)
		}
		b.ImageURL = imageURL
		b.AltText = altText
		b.CampaignID = campaignID
		b.StartDate = startDate
		b.EndDate = endDate
		if err := json.Unmarshal(placementsJSON, &b.Placements); err != nil {
			return nil, 0, fmt.Errorf("unmarshal banner placements: %w", err)
		}
		if err := json.Unmarshal(abVariantsJSON, &b.ABVariants); err != nil {
			return nil, 0, fmt.Errorf("unmarshal banner ab_variants: %w", err)
		}
		banners = append(banners, b)
	}
	return banners, total, nil
}

func (r *bannerRepo) ListActiveBannersForPlacement(contextID, placementID string, date time.Time) ([]*banner.Banner, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at
		FROM banner_banners
		WHERE context_id = $1 AND is_active = true
		  AND ($2 IS NULL OR start_date <= $2) AND ($2 IS NULL OR end_date >= $2)
		ORDER BY priority DESC
	`
	rows, err := r.pool.Query(ctx, query, contextID, date)
	if err != nil {
		return nil, fmt.Errorf("list active banners for placement: %w", err)
	}
	defer rows.Close()

	var banners []*banner.Banner
	for rows.Next() {
		b := &banner.Banner{}
		var placementsJSON, abVariantsJSON []byte
		var imageURL, altText, campaignID *string
		var startDate, endDate *time.Time
		err := rows.Scan(
			&b.ID, &b.ContextID, &b.Name, &b.Type, &b.Content, &b.LinkURL, &imageURL, &altText,
			&campaignID, &placementsJSON, &b.Priority, &startDate, &endDate, &b.IsActive,
			&b.ABTest, &abVariantsJSON, &b.Impressions, &b.Clicks, &b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan banner: %w", err)
		}
		b.ImageURL = imageURL
		b.AltText = altText
		b.CampaignID = campaignID
		b.StartDate = startDate
		b.EndDate = endDate
		if err := json.Unmarshal(placementsJSON, &b.Placements); err != nil {
			return nil, fmt.Errorf("unmarshal banner placements: %w", err)
		}
		if err := json.Unmarshal(abVariantsJSON, &b.ABVariants); err != nil {
			return nil, fmt.Errorf("unmarshal banner ab_variants: %w", err)
		}
		banners = append(banners, b)
	}
	return banners, nil
}

func (r *bannerRepo) UpdateBanner(b *banner.Banner) error {
	ctx := context.Background()
	placementsJSON, err := json.Marshal(b.Placements)
	if err != nil {
		return fmt.Errorf("marshal banner placements: %w", err)
	}
	abVariantsJSON, err := json.Marshal(b.ABVariants)
	if err != nil {
		return fmt.Errorf("marshal banner ab_variants: %w", err)
	}
	query := `
		UPDATE banner_banners
		SET name = $1, type = $2, content = $3, link_url = $4, image_url = $5, alt_text = $6,
		    campaign_id = $7, placements = $8, priority = $9, start_date = $10, end_date = $11,
		    is_active = $12, ab_test = $13, ab_variants = $14, updated_at = NOW()
		WHERE id = $15
	`
	_, err = r.pool.Exec(ctx, query, b.Name, b.Type, b.Content, b.LinkURL, b.ImageURL, b.AltText,
		b.CampaignID, placementsJSON, b.Priority, b.StartDate, b.EndDate, b.IsActive, b.ABTest,
		abVariantsJSON, b.ID)
	if err != nil {
		return fmt.Errorf("update banner: %w", err)
	}
	return nil
}

func (r *bannerRepo) DeleteBanner(id string) error {
	ctx := context.Background()
	query := `DELETE FROM banner_banners WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete banner: %w", err)
	}
	return nil
}

// --- Tracking operations ---

func (r *bannerRepo) RecordImpression(bannerID, visitorID string) error {
	ctx := context.Background()
	query := `
		INSERT INTO banner_impressions (id, banner_id, visitor_id, created_at)
		VALUES ($1, $2, $3, NOW())
	`
	_, err := r.pool.Exec(ctx, query, uuid.New().String(), bannerID, visitorID)
	if err != nil {
		return fmt.Errorf("record impression: %w", err)
	}
	return nil
}

func (r *bannerRepo) RecordClick(bannerID, visitorID string) error {
	ctx := context.Background()
	query := `
		INSERT INTO banner_clicks (id, banner_id, visitor_id, created_at)
		VALUES ($1, $2, $3, NOW())
	`
	_, err := r.pool.Exec(ctx, query, uuid.New().String(), bannerID, visitorID)
	if err != nil {
		return fmt.Errorf("record click: %w", err)
	}
	return nil
}

// --- Campaign operations ---

func (r *bannerRepo) CreateCampaign(c *banner.Campaign) error {
	ctx := context.Background()
	c.ID = uuid.New().String()
	query := `
		INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, 0, 0, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query, c.ID, c.ContextID, c.Name, c.Description,
		c.StartDate, c.EndDate, c.IsActive, c.TargetURL, c.TrackingCode)
	if err != nil {
		return fmt.Errorf("create campaign: %w", err)
	}
	return nil
}

func (r *bannerRepo) GetCampaign(id string) (*banner.Campaign, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at
		FROM banner_campaigns WHERE id = $1
	`
	c := &banner.Campaign{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.ContextID, &c.Name, &c.Description, &c.StartDate, &c.EndDate, &c.IsActive,
		&c.TargetURL, &c.TrackingCode, &c.Impressions, &c.Clicks, &c.ConversionRate, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("campaign not found")
		}
		return nil, fmt.Errorf("get campaign: %w", err)
	}
	return c, nil
}

func (r *bannerRepo) ListCampaigns(contextID string, isActive bool) ([]*banner.Campaign, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at
		FROM banner_campaigns WHERE context_id = $1
	`
	args := []interface{}{contextID}
	argIdx := 2

	if isActive {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*banner.Campaign
	for rows.Next() {
		c := &banner.Campaign{}
		err := rows.Scan(&c.ID, &c.ContextID, &c.Name, &c.Description, &c.StartDate, &c.EndDate, &c.IsActive, &c.TargetURL, &c.TrackingCode, &c.Impressions, &c.Clicks, &c.ConversionRate, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

func (r *bannerRepo) UpdateCampaign(c *banner.Campaign) error {
	ctx := context.Background()
	query := `
		UPDATE banner_campaigns
		SET name = $1, description = $2, start_date = $3, end_date = $4, is_active = $5, target_url = $6, tracking_code = $7, updated_at = NOW()
		WHERE id = $8
	`
	_, err := r.pool.Exec(ctx, query, c.Name, c.Description, c.StartDate, c.EndDate, c.IsActive, c.TargetURL, c.TrackingCode, c.ID)
	if err != nil {
		return fmt.Errorf("update campaign: %w", err)
	}
	return nil
}

func (r *bannerRepo) DeleteCampaign(id string) error {
	ctx := context.Background()
	query := `DELETE FROM banner_campaigns WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete campaign: %w", err)
	}
	return nil
}
