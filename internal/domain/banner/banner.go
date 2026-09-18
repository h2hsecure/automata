package banner

import "time"

type BannerType string

const (
	BannerTypeHTML   BannerType = "html"
	BannerTypeImage  BannerType = "image"
	BannerTypeVideo  BannerType = "video"
	BannerTypePopup  BannerType = "popup"
	BannerTypeSticky BannerType = "sticky"
)

type ABVariant struct {
	ID       string
	Name     string
	Content  string
	LinkURL  string
	Weight   int
	Priority int
}

type Banner struct {
	ID          string
	TenantID    string
	Name        string
	Type        BannerType
	Content     string
	LinkURL     string
	ImageURL    string
	AltText     string
	CampaignID  string
	Placements  []string
	Priority    int
	StartDate   time.Time
	EndDate     time.Time
	IsActive    bool
	ABTest      bool
	ABVariants  []ABVariant
	Impressions int64
	Clicks      int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Placement struct {
	ID          string
	TenantID    string
	Name        string
	Location    string
	CSSSelector string
	MaxBanners  int
	Priority    int
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Campaign struct {
	ID             string
	TenantID       string
	Name           string
	Description    string
	StartDate      time.Time
	EndDate        time.Time
	IsActive       bool
	TargetURL      string
	TrackingCode   string
	Impressions    int64
	Clicks         int64
	ConversionRate float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DateRange struct {
	Start time.Time
	End   time.Time
}

type ListOptions struct {
	CampaignID string
	IsActive   bool
	Offset     int
	Limit      int
}

type CampaignStats struct {
	TotalImpressions int64
	TotalClicks      int64
	ConversionRate   float64
	DailyImpressions []DailyStat
	DailyClicks      []DailyStat
}

type BannerStats struct {
	Impressions int64
	Clicks      int64
	CTR         float64
}

type DailyStat struct {
	Date  time.Time
	Count int64
}

// Impression records a banner impression event.
type Impression struct {
	ID        string
	BannerID  string
	TenantID  string
	VisitorID string
	CreatedAt time.Time
}

// Click records a banner click event.
type Click struct {
	ID        string
	BannerID  string
	TenantID  string
	VisitorID string
	CreatedAt time.Time
}

// CreateBannerRequest is the request body for creating a banner.
type CreateBannerRequest struct {
	Name       string            `json:"name" binding:"required"`
	Type       BannerType        `json:"type"`
	Content    string            `json:"content"`
	LinkURL    string            `json:"link_url"`
	ImageURL   string            `json:"image_url"`
	AltText    string            `json:"alt_text"`
	CampaignID string            `json:"campaign_id"`
	Placements []string          `json:"placements"`
	Priority   int               `json:"priority"`
	StartDate  time.Time         `json:"start_date"`
	EndDate    time.Time         `json:"end_date"`
	ABTest     bool              `json:"ab_test"`
	ABVariants []ABVariant       `json:"ab_variants"`
}

// UpdateBannerRequest is the request body for updating a banner.
type UpdateBannerRequest struct {
	Name       string       `json:"name"`
	Type       BannerType   `json:"type"`
	Content    string       `json:"content"`
	LinkURL    string       `json:"link_url"`
	ImageURL   string       `json:"image_url"`
	AltText    string       `json:"alt_text"`
	CampaignID string       `json:"campaign_id"`
	Placements []string     `json:"placements"`
	Priority   int          `json:"priority"`
	StartDate  time.Time    `json:"start_date"`
	EndDate    time.Time    `json:"end_date"`
	IsActive   *bool        `json:"is_active"`
	ABTest     *bool        `json:"ab_test"`
	ABVariants []ABVariant  `json:"ab_variants"`
}

// CreatePlacementRequest is the request body for creating a placement.
type CreatePlacementRequest struct {
	Name        string `json:"name" binding:"required"`
	Location    string `json:"location"`
	CSSSelector string `json:"css_selector" binding:"required"`
	MaxBanners  int    `json:"max_banners"`
	Priority    int    `json:"priority"`
}

// UpdatePlacementRequest is the request body for updating a placement.
type UpdatePlacementRequest struct {
	Name        string `json:"name"`
	Location    string `json:"location"`
	CSSSelector string `json:"css_selector"`
	MaxBanners  int    `json:"max_banners"`
	Priority    int    `json:"priority"`
	IsActive    *bool  `json:"is_active"`
}

// CreateCampaignRequest is the request body for creating a campaign.
type CreateCampaignRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	TargetURL    string `json:"target_url"`
	TrackingCode string `json:"tracking_code"`
}

// UpdateCampaignRequest is the request body for updating a campaign.
type UpdateCampaignRequest struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	TargetURL    string    `json:"target_url"`
	TrackingCode string    `json:"tracking_code"`
	IsActive     *bool     `json:"is_active"`
}
