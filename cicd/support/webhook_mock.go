package support

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// WebhookRequest represents a captured webhook request.
type WebhookRequest struct {
	URL         string
	Method      string
	Headers     map[string]string
	Payload     map[string]interface{}
	ReceivedAt  time.Time
	StatusCode  int
}

// MockWebhookServer is an embedded test server for capturing webhook deliveries.
type MockWebhookServer struct {
	server    *httptest.Server
	requests  []WebhookRequest
	mu        sync.RWMutex
}

// NewMockWebhookServer creates a new mock webhook server.
func NewMockWebhookServer() *MockWebhookServer {
	mws := &MockWebhookServer{
		requests: make([]WebhookRequest, 0),
	}

	mws.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Capture headers
		headers := make(map[string]string)
		for key, values := range r.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}

		// Store request
		req := WebhookRequest{
			URL:        r.URL.String(),
			Method:     r.Method,
			Headers:    headers,
			Payload:    payload,
			ReceivedAt: time.Now(),
			StatusCode: http.StatusOK,
		}

		mws.mu.Lock()
		mws.requests = append(mws.requests, req)
		mws.mu.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	}))

	return mws
}

// URL returns the mock server URL.
func (mws *MockWebhookServer) URL() string {
	return mws.server.URL
}

// Requests returns a copy of all captured webhook requests.
func (mws *MockWebhookServer) Requests() []WebhookRequest {
	mws.mu.RLock()
	defer mws.mu.RUnlock()

	reqs := make([]WebhookRequest, len(mws.requests))
	copy(reqs, mws.requests)
	return reqs
}

// Clear removes all captured requests.
func (mws *MockWebhookServer) Clear() {
	mws.mu.Lock()
	mws.requests = make([]WebhookRequest, 0)
	mws.mu.Unlock()
}

// Count returns the number of captured requests.
func (mws *MockWebhookServer) Count() int {
	mws.mu.RLock()
	defer mws.mu.RUnlock()
	return len(mws.requests)
}

// HasRequestWithURL checks if a request was received at the specified URL.
func (mws *MockWebhookServer) HasRequestWithURL(url string) bool {
	mws.mu.RLock()
	defer mws.mu.RUnlock()

	for _, req := range mws.requests {
		if req.URL == url {
			return true
		}
	}
	return false
}

// LastRequest returns the most recently captured request.
func (mws *MockWebhookServer) LastRequest() (*WebhookRequest, bool) {
	mws.mu.RLock()
	defer mws.mu.RUnlock()

	if len(mws.requests) == 0 {
		return nil, false
	}

	last := mws.requests[len(mws.requests)-1]
	return &last, true
}

// Close tears down the mock server.
func (mws *MockWebhookServer) Close() {
	mws.server.Close()
}
