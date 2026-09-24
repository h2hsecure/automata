package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	webhookServer *support.MockWebhookServer
)

var _ = BeforeEach(func() {
	webhookServer = support.NewMockWebhookServer()
})

var _ = Describe("Webhook Queue Integration Tests", func() {


	AfterEach(func() {
		// Clean up webhook_deliveries table
		_, err := pool.Exec(ctx, "DELETE FROM webhook_deliveries")
		Expect(err).NotTo(HaveOccurred())
		webhookServer.Clear()
	})

	Describe("Webhook delivery enqueueing", func() {
		It("should enqueue a webhook with pending status", func() {
			tenantID := support.NewTestContextID()
			formID := support.NewTestUUID()
			payload := map[string]interface{}{"key": "value", "form_id": formID}

			err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
			Expect(err).NotTo(HaveOccurred())

			// Verify webhook was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM webhook_deliveries").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Verify status is pending
			var status string
			err = pool.QueryRow(ctx, "SELECT status FROM webhook_deliveries LIMIT 1").Scan(&status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("pending"))
		})

		It("should enqueue webhook with payload data", func() {
			tenantID := support.NewTestContextID()
			formID := support.NewTestUUID()
			payload := map[string]interface{}{
				"contact_email": "test@example.com",
				"contact_name":  "Test User",
				"form_slug":     "contact-form",
			}

			err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
			Expect(err).NotTo(HaveOccurred())

			// Verify payload was stored
			var payloadBytes []byte
			err = pool.QueryRow(ctx, "SELECT payload FROM webhook_deliveries LIMIT 1").Scan(&payloadBytes)
			Expect(err).NotTo(HaveOccurred())

			var storedPayload map[string]interface{}
			err = json.Unmarshal(payloadBytes, &storedPayload)
			Expect(err).NotTo(HaveOccurred())
			Expect(storedPayload["contact_email"]).To(Equal("test@example.com"))
		})
	})

	Describe("Webhook processing", func() {
		It("should process pending webhook and mark as delivered", func() {
			tenantID := support.NewTestContextID()
			formID := support.NewTestUUID()
			payload := map[string]interface{}{"test": "data"}

			err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
			Expect(err).NotTo(HaveOccurred())

			// Get pending webhooks
			webhookQueue := queue.NewWebhookQueue(pool)
			deliveries, err := webhookQueue.GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(1))

			// Simulate successful delivery
			err = webhookQueue.MarkSuccess(deliveries[0].ID)
			Expect(err).NotTo(HaveOccurred())

			// Verify status is success
			var status string
			err = pool.QueryRow(ctx, "SELECT status FROM webhook_deliveries WHERE id = $1", deliveries[0].ID).Scan(&status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("success"))
		})

		It("should retry failed webhooks up to max_attempts", func() {
			tenantID := support.NewTestContextID()
			formID := support.NewTestUUID()
			payload := map[string]interface{}{"test": "data"}

			err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
			Expect(err).NotTo(HaveOccurred())

			webhookQueue := queue.NewWebhookQueue(pool)
			deliveries, err := webhookQueue.GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(1))

			// Simulate failure and retry
			nextRetry := time.Now().Add(1 * time.Minute)
			err = webhookQueue.UpdateNextRetry(deliveries[0].ID, nextRetry)
			Expect(err).NotTo(HaveOccurred())

			// Verify attempts were incremented
			var attempts int
			err = pool.QueryRow(ctx, "SELECT attempts FROM webhook_deliveries WHERE id = $1", deliveries[0].ID).Scan(&attempts)
			Expect(err).NotTo(HaveOccurred())
			Expect(attempts).To(Equal(1))
		})

		It("should mark webhook as permanently failed after max_attempts exceeded", func() {
			tenantID := support.NewTestContextID()
			formID := support.NewTestUUID()
			payload := map[string]interface{}{"test": "data"}

			err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
			Expect(err).NotTo(HaveOccurred())

			webhookQueue := queue.NewWebhookQueue(pool)
			deliveries, err := webhookQueue.GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(1))

			// Simulate permanent failure
			errMsg := "Connection timeout after 30s"
			err = webhookQueue.MarkFailed(deliveries[0].ID, errMsg)
			Expect(err).NotTo(HaveOccurred())

			// Verify status is failed
			var status string
			var errorMsg string
			err = pool.QueryRow(ctx, "SELECT status, error_msg FROM webhook_deliveries WHERE id = $1", deliveries[0].ID).Scan(&status, &errorMsg)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("failed"))
			Expect(errorMsg).To(ContainSubstring(errMsg))
		})
	})

	Describe("Mock webhook capture", func() {
		It("should capture delivered webhooks in mock server", func() {
			tenantID := support.NewTestContextID()
			formID := support.NewTestUUID()
			payload := map[string]interface{}{"contact_email": "test@example.com"}

			err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
			Expect(err).NotTo(HaveOccurred())

			// Get and process webhook
			webhookQueue := queue.NewWebhookQueue(pool)
			deliveries, err := webhookQueue.GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(1))

			// Simulate delivery by making actual HTTP request
			client := &http.Client{}
			reqBody, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", webhookServer.URL(), bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp.Body.Close()

			// Mark as success
			err = webhookQueue.MarkSuccess(deliveries[0].ID)
			Expect(err).NotTo(HaveOccurred())

			// Verify webhook was captured
			Expect(webhookServer.Count()).To(Equal(1))
			reqs := webhookServer.Requests()
			Expect(reqs).To(HaveLen(1))
			Expect(reqs[0].Payload["contact_email"]).To(Equal("test@example.com"))
		})

		It("should capture multiple webhook deliveries", func() {
			tenantID := support.NewTestContextID()

			// Enqueue multiple webhooks
			for i := 0; i < 3; i++ {
				formID := support.NewTestUUID()
				payload := map[string]interface{}{"index": i}
				err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get pending webhooks
			webhookQueue := queue.NewWebhookQueue(pool)
			deliveries, err := webhookQueue.GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(3))

			// Process all
			for _, d := range deliveries {
				err = webhookQueue.MarkSuccess(d.ID)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify all were marked as delivered
			for _, d := range deliveries {
				var status string
				err := pool.QueryRow(ctx, "SELECT status FROM webhook_deliveries WHERE id = $1", d.ID).Scan(&status)
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal("success"))
			}
		})
	})

	Describe("Webhook listing", func() {
		It("should list pending webhooks for worker processing", func() {
			tenantID := support.NewTestContextID()

			// Enqueue multiple webhooks
			for i := 0; i < 5; i++ {
				formID := support.NewTestUUID()
				payload := map[string]interface{}{"index": i}
				err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get pending webhooks with limit
			webhookQueue := queue.NewWebhookQueue(pool)
			deliveries, err := webhookQueue.GetPending(3)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(3))
		})

		It("should only return pending webhooks", func() {
			tenantID := support.NewTestContextID()
			formID := support.NewTestUUID()

			// Enqueue a webhook
			payload := map[string]interface{}{"test": "data"}
			err := queue.NewWebhookQueue(pool).Enqueue(webhookServer.URL(), payload, tenantID, formID)
			Expect(err).NotTo(HaveOccurred())

			// Mark one as success
			webhookQueue := queue.NewWebhookQueue(pool)
			deliveries, err := webhookQueue.GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(1))

			err = webhookQueue.MarkSuccess(deliveries[0].ID)
			Expect(err).NotTo(HaveOccurred())

			// Get pending webhooks - should be empty
			deliveries, err = webhookQueue.GetPending(10)
			Expect(err).NotTo(HaveOccurred())
			Expect(deliveries).To(HaveLen(0))
		})
	})
})
