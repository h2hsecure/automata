package support

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Email represents a captured email message.
type Email struct {
	To        []string
	Subject   string
	Body      string
	HTMLBody  string
	SentAt    time.Time
}

// MockSMTP is an in-memory mock SMTP server for testing email delivery.
type MockSMTP struct {
	mu      sync.RWMutex
	emails  []Email
	listener string
	running bool
}

// NewMockSMTP creates a new mock SMTP server.
func NewMockSMTP() *MockSMTP {
	return &MockSMTP{
		emails: make([]Email, 0),
		listener: "127.0.0.1:0",
	}
}

// Start begins listening for emails (no-op for in-memory mock).
func (m *MockSMTP) Start() error {
	m.running = true
	return nil
}

// Stop stops the mock SMTP server.
func (m *MockSMTP) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
}

// Send simulates sending an email and captures it in memory.
func (m *MockSMTP) Send(to []string, subject, body, htmlBody string) error {
	if !m.running {
		return fmt.Errorf("mock SMTP not running")
	}

	email := Email{
		To:       to,
		Subject:  subject,
		Body:     body,
		HTMLBody: htmlBody,
		SentAt:   time.Now(),
	}

	m.mu.Lock()
	m.emails = append(m.emails, email)
	m.mu.Unlock()

	return nil
}

// Emails returns a copy of all captured emails.
func (m *MockSMTP) Emails() []Email {
	m.mu.RLock()
	defer m.mu.RUnlock()

	emails := make([]Email, len(m.emails))
	copy(emails, m.emails)
	return emails
}

// Clear removes all captured emails.
func (m *MockSMTP) Clear() {
	m.mu.Lock()
	m.emails = make([]Email, 0)
	m.mu.Unlock()
}

// Count returns the number of captured emails.
func (m *MockSMTP) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.emails)
}

// HasEmailTo checks if an email was sent to the specified address.
func (m *MockSMTP) HasEmailTo(to string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, email := range m.emails {
		for _, recipient := range email.To {
			if recipient == to {
				return true
			}
		}
	}
	return false
}

// HasEmailWithSubject checks if an email with the specified subject exists.
func (m *MockSMTP) HasEmailWithSubject(subject string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, email := range m.emails {
		if email.Subject == subject {
			return true
		}
	}
	return false
}

// LastEmail returns the most recently captured email.
func (m *MockSMTP) LastEmail() (*Email, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.emails) == 0 {
		return nil, false
	}

	last := m.emails[len(m.emails)-1]
	return &last, true
}

// SMTPDialer provides a mock smtp.Dial implementation.
type SMTPDialer struct {
	Mock *MockSMTP
}

// Dial returns a mock SMTP client.
func (d *SMTPDialer) Dial() (*SMTPClient, error) {
	return &SMTPClient{Mock: d.Mock}, nil
}

// SMTPClient is a mock smtp.Client implementation.
type SMTPClient struct {
	Mock *MockSMTP
}

// Mail implements smtp.Client.Mail.
func (c *SMTPClient) Mail(from string) error {
	return nil
}

// Rcpt implements smtp.Client.Rcpt.
func (c *SMTPClient) Rcpt(to string) error {
	return nil
}

// Close implements smtp.Client.Close.
func (c *SMTPClient) Close() error {
	return nil
}

// Data implements smtp.Client.Data.
func (c *SMTPClient) Data(body []byte) error {
	// Parse simple email format
	lines := strings.Split(string(body), "\n")
	to := []string{}
	subject := ""
	bodyText := ""
	inHeaders := true

	for _, line := range lines {
		if inHeaders {
			if line == "" {
				inHeaders = false
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "to:") {
				addr := strings.TrimPrefix(strings.ToLower(line), "to:")
				addr = strings.TrimSpace(addr)
				to = append(to, addr)
			}
			if strings.HasPrefix(strings.ToLower(line), "subject:") {
				subject = strings.TrimPrefix(strings.ToLower(line), "subject:")
				subject = strings.TrimSpace(subject)
			}
		} else {
			bodyText += line + "\n"
		}
	}

	_ = c.Mock.Send(to, subject, bodyText, "")
	return nil
}

// NewSMTPDialer creates a new SMTP dialer for the mock SMTP.
func NewSMTPDialer(mock *MockSMTP) *SMTPDialer {
	return &SMTPDialer{Mock: mock}
}
