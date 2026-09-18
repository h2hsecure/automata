package banner

import "time"

// Repository defines the interface for banner data persistence.
type Repository interface {
	// Placement operations
	CreatePlacement(p *Placement) error
	GetPlacement(id string) (*Placement, error)
	ListPlacements(tenantID string, isActive bool) ([]*Placement, error)
	UpdatePlacement(p *Placement) error
	DeletePlacement(id string) error

	// Banner operations
	CreateBanner(b *Banner) error
	GetBanner(id string) (*Banner, error)
	ListBanners(tenantID string, opts ListOptions) ([]*Banner, int64, error)
	ListActiveBannersForPlacement(tenantID, placementID string, date time.Time) ([]*Banner, error)
	UpdateBanner(b *Banner) error
	DeleteBanner(id string) error

	// Tracking operations
	RecordImpression(bannerID, visitorID string) error
	RecordClick(bannerID, visitorID string) error

	// Campaign operations
	CreateCampaign(c *Campaign) error
	GetCampaign(id string) (*Campaign, error)
	ListCampaigns(tenantID string, isActive bool) ([]*Campaign, error)
	UpdateCampaign(c *Campaign) error
	DeleteCampaign(id string) error
}
