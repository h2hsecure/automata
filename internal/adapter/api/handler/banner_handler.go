package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	bdomain "github.com/hekemen/automata/internal/domain/banner"
	bannerUsecase "github.com/hekemen/automata/internal/usecase/banner"
)

// BannerHandler handles banner CRUD API endpoints.
type BannerHandler struct {
	repo bdomain.Repository
}

// NewBannerHandler creates a new BannerHandler.
func NewBannerHandler(repo bdomain.Repository) *BannerHandler {
	return &BannerHandler{repo: repo}
}

// CreateBanner handles POST /api/banners — creates a new banner.
func (h *BannerHandler) CreateBanner(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	var req bdomain.CreateBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	banner, err := bannerUsecase.NewBannerUsecase(h.repo).CreateBanner(contextID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, banner)
}

// UpdateBanner handles PUT /api/banners/:id — updates an existing banner.
func (h *BannerHandler) UpdateBanner(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	bannerID := c.Param("id")

	var req bdomain.UpdateBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	banner, err := bannerUsecase.NewBannerUsecase(h.repo).UpdateBanner(contextID, bannerID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, banner)
}

// DeleteBanner handles DELETE /api/banners/:id — deletes a banner.
func (h *BannerHandler) DeleteBanner(c *gin.Context) {
	bannerID := c.Param("id")

	if err := bannerUsecase.NewBannerUsecase(h.repo).DeleteBanner(bannerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "banner deleted"})
}

// GetBanner handles GET /api/banners/:id — retrieves a banner.
func (h *BannerHandler) GetBanner(c *gin.Context) {
	bannerID := c.Param("id")

	banner, err := bannerUsecase.NewBannerUsecase(h.repo).GetBanner(bannerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "banner not found"})
		return
	}

	c.JSON(http.StatusOK, banner)
}

// ListBanners handles GET /api/banners — lists banners with optional filters.
func (h *BannerHandler) ListBanners(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	var opts bdomain.ListOptions

	if campaignID := c.Query("campaign_id"); campaignID != "" {
		opts.CampaignID = campaignID
	}
	if isActive := c.Query("is_active"); isActive != "" {
		if isActive == "true" {
			opts.IsActive = true
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if p, err := strconv.Atoi(offset); err == nil {
			opts.Offset = p
		}
	}
	if limit := c.Query("limit"); limit != "" {
		if p, err := strconv.Atoi(limit); err == nil {
			opts.Limit = p
		}
	}

	banners, total, err := bannerUsecase.NewBannerUsecase(h.repo).ListBanners(contextID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"banners": banners,
		"total":   total,
	})
}

// PlacementHandler handles placement CRUD API endpoints.
type PlacementHandler struct {
	repo bdomain.Repository
}

// NewPlacementHandler creates a new PlacementHandler.
func NewPlacementHandler(repo bdomain.Repository) *PlacementHandler {
	return &PlacementHandler{repo: repo}
}

// CreatePlacement handles POST /api/placements — creates a new placement.
func (h *PlacementHandler) CreatePlacement(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	var req bdomain.CreatePlacementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	placement, err := bannerUsecase.NewBannerUsecase(h.repo).CreatePlacement(contextID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, placement)
}

// UpdatePlacement handles PUT /api/placements/:id — updates an existing placement.
func (h *PlacementHandler) UpdatePlacement(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	placementID := c.Param("id")

	var req bdomain.UpdatePlacementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	placement, err := bannerUsecase.NewBannerUsecase(h.repo).UpdatePlacement(contextID, placementID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, placement)
}

// DeletePlacement handles DELETE /api/placements/:id — deletes a placement.
func (h *PlacementHandler) DeletePlacement(c *gin.Context) {
	placementID := c.Param("id")

	if err := bannerUsecase.NewBannerUsecase(h.repo).DeletePlacement(placementID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "placement deleted"})
}

// GetPlacement handles GET /api/placements/:id — retrieves a placement.
func (h *PlacementHandler) GetPlacement(c *gin.Context) {
	placementID := c.Param("id")

	placement, err := bannerUsecase.NewBannerUsecase(h.repo).GetPlacement(placementID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "placement not found"})
		return
	}

	c.JSON(http.StatusOK, placement)
}

// ListPlacements handles GET /api/placements — lists placements with optional filters.
func (h *PlacementHandler) ListPlacements(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	isActive := false
	if active := c.Query("is_active"); active == "true" {
		isActive = true
	}

	placements, err := bannerUsecase.NewBannerUsecase(h.repo).ListPlacements(contextID, isActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"placements": placements,
	})
}

// CampaignHandler handles campaign CRUD API endpoints.
type CampaignHandler struct {
	repo bdomain.Repository
}

// NewCampaignHandler creates a new CampaignHandler.
func NewCampaignHandler(repo bdomain.Repository) *CampaignHandler {
	return &CampaignHandler{repo: repo}
}

// CreateCampaign handles POST /api/campaigns — creates a new campaign.
func (h *CampaignHandler) CreateCampaign(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	var req bdomain.CreateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	campaign, err := bannerUsecase.NewBannerUsecase(h.repo).CreateCampaign(contextID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

// UpdateCampaign handles PUT /api/campaigns/:id — updates an existing campaign.
func (h *CampaignHandler) UpdateCampaign(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	campaignID := c.Param("id")

	var req bdomain.UpdateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	campaign, err := bannerUsecase.NewBannerUsecase(h.repo).UpdateCampaign(contextID, campaignID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// DeleteCampaign handles DELETE /api/campaigns/:id — deletes a campaign.
func (h *CampaignHandler) DeleteCampaign(c *gin.Context) {
	campaignID := c.Param("id")

	if err := bannerUsecase.NewBannerUsecase(h.repo).DeleteCampaign(campaignID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "campaign deleted"})
}

// GetCampaign handles GET /api/campaigns/:id — retrieves a campaign.
func (h *CampaignHandler) GetCampaign(c *gin.Context) {
	campaignID := c.Param("id")

	campaign, err := bannerUsecase.NewBannerUsecase(h.repo).GetCampaign(campaignID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// ListCampaigns handles GET /api/campaigns — lists campaigns with optional filters.
func (h *CampaignHandler) ListCampaigns(c *gin.Context) {
	contextID := c.GetHeader("X-Context-ID")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
		return
	}

	isActive := false
	if active := c.Query("is_active"); active == "true" {
		isActive = true
	}

	campaigns, err := bannerUsecase.NewBannerUsecase(h.repo).ListCampaigns(contextID, isActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"campaigns": campaigns,
	})
}
