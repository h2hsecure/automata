package banner

import (
	"fmt"
	"time"

	bdomain "github.com/hekemen/automata/internal/domain/banner"
)

// BannerUsecase handles banner business logic.
type BannerUsecase struct {
	repo bdomain.Repository
}

// NewBannerUsecase creates a new BannerUsecase.
func NewBannerUsecase(repo bdomain.Repository) *BannerUsecase {
	return &BannerUsecase{repo: repo}
}

// CreateBanner creates a new banner.
func (u *BannerUsecase) CreateBanner(tenantID string, req *bdomain.CreateBannerRequest) (*bdomain.Banner, error) {
	b := &bdomain.Banner{
		TenantID:   tenantID,
		Name:       req.Name,
		Type:       req.Type,
		Content:    req.Content,
		LinkURL:    req.LinkURL,
		ImageURL:   req.ImageURL,
		AltText:    req.AltText,
		CampaignID: req.CampaignID,
		Placements: req.Placements,
		Priority:   req.Priority,
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
		IsActive:   true,
		ABTest:     req.ABTest,
		ABVariants: req.ABVariants,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := u.repo.CreateBanner(b); err != nil {
		return nil, fmt.Errorf("create banner: %w", err)
	}

	return b, nil
}

// UpdateBanner updates an existing banner.
func (u *BannerUsecase) UpdateBanner(tenantID, bannerID string, req *bdomain.UpdateBannerRequest) (*bdomain.Banner, error) {
	b, err := u.repo.GetBanner(bannerID)
	if err != nil {
		return nil, fmt.Errorf("banner not found: %w", err)
	}

	if req.Name != "" {
		b.Name = req.Name
	}
	if req.Type != "" {
		b.Type = req.Type
	}
	if req.Content != "" {
		b.Content = req.Content
	}
	if req.LinkURL != "" {
		b.LinkURL = req.LinkURL
	}
	if req.ImageURL != "" {
		b.ImageURL = req.ImageURL
	}
	if req.AltText != "" {
		b.AltText = req.AltText
	}
	if req.CampaignID != "" {
		b.CampaignID = req.CampaignID
	}
	if req.Placements != nil {
		b.Placements = req.Placements
	}
	if req.Priority > 0 {
		b.Priority = req.Priority
	}
	if !req.StartDate.IsZero() {
		b.StartDate = req.StartDate
	}
	if !req.EndDate.IsZero() {
		b.EndDate = req.EndDate
	}
	if req.IsActive != nil {
		b.IsActive = *req.IsActive
	}
	if req.ABTest != nil {
		b.ABTest = *req.ABTest
	}
	if req.ABVariants != nil {
		b.ABVariants = req.ABVariants
	}
	b.UpdatedAt = time.Now()

	if err := u.repo.UpdateBanner(b); err != nil {
		return nil, fmt.Errorf("update banner: %w", err)
	}

	return b, nil
}

// DeleteBanner deletes a banner.
func (u *BannerUsecase) DeleteBanner(bannerID string) error {
	if err := u.repo.DeleteBanner(bannerID); err != nil {
		return fmt.Errorf("delete banner: %w", err)
	}
	return nil
}

// GetBanner retrieves a banner by ID.
func (u *BannerUsecase) GetBanner(bannerID string) (*bdomain.Banner, error) {
	b, err := u.repo.GetBanner(bannerID)
	if err != nil {
		return nil, fmt.Errorf("get banner: %w", err)
	}
	return b, nil
}

// ListBanners lists banners with optional filters.
func (u *BannerUsecase) ListBanners(tenantID string, opts bdomain.ListOptions) ([]*bdomain.Banner, int64, error) {
	banners, total, err := u.repo.ListBanners(tenantID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list banners: %w", err)
	}
	return banners, total, nil
}

// CreatePlacement creates a new placement.
func (u *BannerUsecase) CreatePlacement(tenantID string, req *bdomain.CreatePlacementRequest) (*bdomain.Placement, error) {
	p := &bdomain.Placement{
		TenantID:    tenantID,
		Name:        req.Name,
		Location:    req.Location,
		CSSSelector: req.CSSSelector,
		MaxBanners:  req.MaxBanners,
		Priority:    req.Priority,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := u.repo.CreatePlacement(p); err != nil {
		return nil, fmt.Errorf("create placement: %w", err)
	}

	return p, nil
}

// UpdatePlacement updates an existing placement.
func (u *BannerUsecase) UpdatePlacement(tenantID, placementID string, req *bdomain.UpdatePlacementRequest) (*bdomain.Placement, error) {
	p, err := u.repo.GetPlacement(placementID)
	if err != nil {
		return nil, fmt.Errorf("placement not found: %w", err)
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Location != "" {
		p.Location = req.Location
	}
	if req.CSSSelector != "" {
		p.CSSSelector = req.CSSSelector
	}
	if req.MaxBanners > 0 {
		p.MaxBanners = req.MaxBanners
	}
	if req.Priority > 0 {
		p.Priority = req.Priority
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}
	p.UpdatedAt = time.Now()

	if err := u.repo.UpdatePlacement(p); err != nil {
		return nil, fmt.Errorf("update placement: %w", err)
	}

	return p, nil
}

// DeletePlacement deletes a placement.
func (u *BannerUsecase) DeletePlacement(placementID string) error {
	if err := u.repo.DeletePlacement(placementID); err != nil {
		return fmt.Errorf("delete placement: %w", err)
	}
	return nil
}

// GetPlacement retrieves a placement by ID.
func (u *BannerUsecase) GetPlacement(placementID string) (*bdomain.Placement, error) {
	p, err := u.repo.GetPlacement(placementID)
	if err != nil {
		return nil, fmt.Errorf("get placement: %w", err)
	}
	return p, nil
}

// ListPlacements lists placements with optional active filter.
func (u *BannerUsecase) ListPlacements(tenantID string, isActive bool) ([]*bdomain.Placement, error) {
	placements, err := u.repo.ListPlacements(tenantID, isActive)
	if err != nil {
		return nil, fmt.Errorf("list placements: %w", err)
	}
	return placements, nil
}

// CreateCampaign creates a new campaign.
func (u *BannerUsecase) CreateCampaign(tenantID string, req *bdomain.CreateCampaignRequest) (*bdomain.Campaign, error) {
	c := &bdomain.Campaign{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		TargetURL:   req.TargetURL,
		TrackingCode: req.TrackingCode,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := u.repo.CreateCampaign(c); err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}

	return c, nil
}

// UpdateCampaign updates an existing campaign.
func (u *BannerUsecase) UpdateCampaign(tenantID, campaignID string, req *bdomain.UpdateCampaignRequest) (*bdomain.Campaign, error) {
	c, err := u.repo.GetCampaign(campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign not found: %w", err)
	}

	if req.Name != "" {
		c.Name = req.Name
	}
	if req.Description != "" {
		c.Description = req.Description
	}
	if !req.StartDate.IsZero() {
		c.StartDate = req.StartDate
	}
	if !req.EndDate.IsZero() {
		c.EndDate = req.EndDate
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}
	if req.TargetURL != "" {
		c.TargetURL = req.TargetURL
	}
	if req.TrackingCode != "" {
		c.TrackingCode = req.TrackingCode
	}
	c.UpdatedAt = time.Now()

	if err := u.repo.UpdateCampaign(c); err != nil {
		return nil, fmt.Errorf("update campaign: %w", err)
	}

	return c, nil
}

// DeleteCampaign deletes a campaign.
func (u *BannerUsecase) DeleteCampaign(campaignID string) error {
	if err := u.repo.DeleteCampaign(campaignID); err != nil {
		return fmt.Errorf("delete campaign: %w", err)
	}
	return nil
}

// GetCampaign retrieves a campaign by ID.
func (u *BannerUsecase) GetCampaign(campaignID string) (*bdomain.Campaign, error) {
	c, err := u.repo.GetCampaign(campaignID)
	if err != nil {
		return nil, fmt.Errorf("get campaign: %w", err)
	}
	return c, nil
}

// ListCampaigns lists campaigns with optional active filter.
func (u *BannerUsecase) ListCampaigns(tenantID string, isActive bool) ([]*bdomain.Campaign, error) {
	campaigns, err := u.repo.ListCampaigns(tenantID, isActive)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	return campaigns, nil
}

// RecordImpression records a banner impression.
func (u *BannerUsecase) RecordImpression(bannerID, visitorID string) error {
	if err := u.repo.RecordImpression(bannerID, visitorID); err != nil {
		return fmt.Errorf("record impression: %w", err)
	}
	return nil
}

// RecordClick records a banner click.
func (u *BannerUsecase) RecordClick(bannerID, visitorID string) error {
	if err := u.repo.RecordClick(bannerID, visitorID); err != nil {
		return fmt.Errorf("record click: %w", err)
	}
	return nil
}
