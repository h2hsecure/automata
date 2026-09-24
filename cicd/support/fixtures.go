package support

import (
	"time"

	"github.com/google/uuid"
)

// NewTestUUID generates a random UUID for test data.
func NewTestUUID() string {
	return uuid.New().String()
}

// NewTestContextID generates a random context ID for tests.
func NewTestContextID() string {
	return uuid.New().String()
}

// NewTestContact generates a test contact.
func NewTestContact(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":           uuid.New().String(),
		"context_id":   contextID,
		"email":        "test@example.com",
		"first_name":   "Test",
		"last_name":    "User",
		"phone":        "+1234567890",
		"company":      "Test Corp",
		"custom_fields": map[string]interface{}{},
		"source":       "test",
		"source_id":    "",
		"created_at":   time.Now().UTC(),
		"updated_at":   time.Now().UTC(),
	}
}

// NewTestForm generates a test form.
func NewTestForm(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":          uuid.New().String(),
		"context_id":  contextID,
		"slug":        "test-form",
		"name":        "Test Form",
		"description": "A test form",
		"fields":      []map[string]interface{}{{"name": "email", "type": "email", "required": true}},
		"settings":    map[string]interface{}{},
		"created_at":  time.Now().UTC(),
		"updated_at":  time.Now().UTC(),
	}
}

// NewTestVisitor generates a test visitor.
func NewTestVisitor(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":           uuid.New().String(),
		"context_id":   contextID,
		"cookie_value": "test-cookie-" + uuid.New().String(),
		"fingerprint":  "test-fingerprint",
		"first_seen":   time.Now().UTC(),
		"last_seen":    time.Now().UTC(),
		"page_views":   0,
	}
}

// NewTestBanner generates a test banner.
func NewTestBanner(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"context_id":  contextID,
		"name":       "Test Banner",
		"type":       "html",
		"content":    "<div>Test</div>",
		"link_url":   "https://example.com",
		"image_url":  "",
		"alt_text":   "Test Banner",
		"campaign_id": uuid.New().String(),
		"placements": []string{"header"},
		"priority":   1,
		"start_date": time.Now().Add(-24 * time.Hour).UTC(),
		"end_date":   time.Now().Add(24 * time.Hour).UTC(),
		"is_active":  true,
		"ab_test":    false,
		"ab_variants": []map[string]interface{}{},
		"impressions":  0,
		"clicks":      0,
		"created_at":  time.Now().UTC(),
		"updated_at":  time.Now().UTC(),
	}
}

// NewTestCampaign generates a test campaign.
func NewTestCampaign(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":            uuid.New().String(),
		"context_id":     contextID,
		"name":          "Test Campaign",
		"description":   "A test campaign",
		"start_date":    time.Now().Add(-24 * time.Hour).UTC(),
		"end_date":      time.Now().Add(24 * time.Hour).UTC(),
		"is_active":     true,
		"target_url":    "https://example.com",
		"tracking_code": "test_campaign",
		"impressions":   0,
		"clicks":        0,
		"conversion_rate": 0.0,
		"created_at":    time.Now().UTC(),
		"updated_at":    time.Now().UTC(),
	}
}

// NewTestPlacement generates a test placement.
func NewTestPlacement(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":          uuid.New().String(),
		"context_id":   contextID,
		"name":        "Test Placement",
		"location":    "header",
		"css_selector": "#header",
		"max_banners": 1,
		"priority":    1,
		"is_active":   true,
		"created_at":  time.Now().UTC(),
		"updated_at":  time.Now().UTC(),
	}
}

// NewTestEmailJob generates a test email job.
func NewTestEmailJob(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":          uuid.New().String(),
		"context_id":   contextID,
		"to_addresses": []string{"recipient@example.com"},
		"subject":     "Test Email",
		"body":        "Test email body",
		"html_body":   "<p>Test email body</p>",
		"attempts":    0,
		"max_retries": 3,
		"next_retry":  time.Now().UTC(),
		"created_at":  time.Now().UTC(),
	}
}

// NewTestWebhookDelivery generates a test webhook delivery.
func NewTestWebhookDelivery(contextID, formID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"context_id":  contextID,
		"form_id":    formID,
		"url":        "https://example.com/webhook",
		"payload":    map[string]interface{}{"key": "value"},
		"status":     "pending",
		"attempts":   0,
		"max_retries": 3,
		"next_retry": time.Now().UTC(),
		"error_msg":  "",
		"created_at": time.Now().UTC(),
		"updated_at": time.Now().UTC(),
	}
}

// NewTestContext generates a test context.
func NewTestContext() map[string]interface{} {
	return map[string]interface{}{
		"id":        uuid.New().String(),
		"slug":      "test-context-" + uuid.New().String()[:8],
		"name":      "Test Context",
		"domain":    "test.example.com",
		"is_active": true,
		"settings":  map[string]interface{}{},
		"created_at": time.Now().UTC(),
		"updated_at": time.Now().UTC(),
	}
}

// NewTestFormSubmission generates a test form submission.
func NewTestFormSubmission(formID, contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":        uuid.New().String(),
		"form_id":   formID,
		"context_id": contextID,
		"data":      map[string]interface{}{"email": "user@example.com"},
		"files":     []map[string]interface{}{},
		"created_at": time.Now().UTC(),
	}
}

// NewTestEvent generates a test tracking event.
func NewTestEvent(contextID, visitorID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"context_id":  contextID,
		"visitor_id": visitorID,
		"type":       "pageview",
		"url":        "https://example.com",
		"title":      "Test Page",
		"referrer":   "",
		"event_name": "",
		"properties": map[string]interface{}{},
		"user_agent": "TestBrowser/1.0",
		"ip_hash":    "test-ip-hash",
		"utm_source": "",
		"utm_medium": "",
		"utm_campaign": "",
		"created_at": time.Now().UTC(),
	}
}

// NewTestTag generates a test tag.
func NewTestTag(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"context_id":  contextID,
		"name":       "test-tag",
		"color":      "#6366f1",
		"created_at": time.Now().UTC(),
	}
}

// NewTestContactTagMembership generates a test contact-tag membership.
func NewTestContactTagMembership(contactID, tagID string) map[string]interface{} {
	return map[string]interface{}{
		"contact_id": contactID,
		"tag_id":     tagID,
	}
}

// NewTestFieldDefinition generates a test field definition.
func NewTestFieldDefinition(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"context_id":  contextID,
		"key":        "custom_field",
		"label":      "Custom Field",
		"type":       "text",
		"options":    map[string]interface{}{},
		"created_at": time.Now().UTC(),
	}
}

// NewTestBannerImpression generates a test banner impression.
func NewTestBannerImpression(bannerID, contextID, visitorID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"banner_id":  bannerID,
		"context_id":  contextID,
		"visitor_id": visitorID,
		"placement_id": uuid.New().String(),
		"created_at": time.Now().UTC(),
	}
}

// NewTestBannerClick generates a test banner click.
func NewTestBannerClick(bannerID, contextID, visitorID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"banner_id":  bannerID,
		"context_id":  contextID,
		"visitor_id": visitorID,
		"created_at": time.Now().UTC(),
	}
}

// NewTestTenantUser generates a test context user.
func NewTestTenantUser(contextID string) map[string]interface{} {
	return map[string]interface{}{
		"id":           uuid.New().String(),
		"context_id":    contextID,
		"email":        "admin@example.com",
		"password_hash": "$2a$10$testhash",
		"is_owner":     true,
		"created_at":   time.Now().UTC(),
		"updated_at":   time.Now().UTC(),
	}
}

// NewTestAPIKey generates a test API key.
func NewTestAPIKey(contextID, userID string) map[string]interface{} {
	return map[string]interface{}{
		"id":         uuid.New().String(),
		"context_id":  contextID,
		"user_id":    userID,
		"key_hash":   "test-key-hash",
		"name":       "test-key",
		"expires_at": time.Now().Add(24 * 365 * time.Hour).UTC(),
		"created_at": time.Now().UTC(),
	}
}
