package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ehhook "github.com/hekemen/automata/internal/domain/webhook"
	"github.com/hekemen/automata/internal/infrastructure/webhook"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// WebhookHandler handles HTTP requests for webhook endpoint management.
type WebhookHandler struct {
	service *webhook.Service
	log     zerolog.Logger
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(svc *webhook.Service) *WebhookHandler {
	return &WebhookHandler{
		service: svc,
		log:     log.With().Str("handler", "webhook").Logger(),
	}
}

// List handles GET /api/admin/webhooks — list webhook endpoints.
func (h *WebhookHandler) List(c *gin.Context) {
	contextID := c.Query("context_id")

	if contextID != "" {
		if _, err := uuid.Parse(contextID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_id format"})
			return
		}

		endpoints, err := h.service.Repo().ListByContext(contextID)
		if err != nil {
			log.Error().Err(err).Msg("failed to list webhook endpoints")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhooks"})
			return
		}

		resp := &ehhook.WebhookListResponse{
			Webhooks: make([]*ehhook.WebhookResponse, 0, len(endpoints)),
			Total:    len(endpoints),
		}

		for _, e := range endpoints {
			resp.Webhooks = append(resp.Webhooks, webhook.ToResponse(e))
		}

		c.JSON(http.StatusOK, resp)
		return
	}

	// If no context_id, return all (super-admin) — for now list an empty response
	// since we don't have a ListAll method
	c.JSON(http.StatusOK, &ehhook.WebhookListResponse{
		Webhooks: []*ehhook.WebhookResponse{},
		Total:    0,
	})
}

// Create handles POST /api/admin/webhooks — create a webhook endpoint.
func (h *WebhookHandler) Create(c *gin.Context) {
	var input ehhook.CreateWebhookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	result, err := h.service.CreateEndpoint(ctx, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// Get handles GET /api/admin/webhooks/:id — get a webhook endpoint by ID.
func (h *WebhookHandler) Get(c *gin.Context) {
	id := c.Param("id")

	endpoint, err := h.service.Repo().GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook endpoint not found"})
		return
	}

	c.JSON(http.StatusOK, webhook.ToResponse(endpoint))
}

// Update handles PUT /api/admin/webhooks/:id — update a webhook endpoint.
func (h *WebhookHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var input ehhook.UpdateWebhookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	result, err := h.service.UpdateEndpoint(ctx, id, input)
	if err != nil {
		if err.Error() == "webhook endpoint not found: sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "webhook endpoint not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Delete handles DELETE /api/admin/webhooks/:id — delete a webhook endpoint.
func (h *WebhookHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Repo().Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook endpoint not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "webhook endpoint deleted"})
}

// TestDeliver handles POST /api/admin/webhooks/:id/test — test delivery.
func (h *WebhookHandler) TestDeliver(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	deliveryID, err := h.service.DeliverTest(ctx, id)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "webhook endpoint not found: sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "webhook endpoint not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"delivery_id": deliveryID,
		"message":     "test delivery enqueued",
	})
}

// GetRepo returns the webhook repository (for internal access).
func (h *WebhookHandler) GetRepo() ehhook.Repository {
	return h.service.Repo()
}

// parsePage extracts page number from query params.
func parsePage(c *gin.Context) int {
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			return v
		}
	}
	return 1
}
