package smtp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	stdsmtp "net/smtp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	StartTLS bool
}

func (c Config) IsConfigured() bool { return c.Host != "" && c.From != "" }

type Client interface {
	Mail(from string) error
	Rcpt(to string) error
	Data(data []byte) error
	Close() error
}

type Dialer interface {
	Dial() (Client, error)
}

type SMTPMailer struct {
	cfg    Config
	dialer Dialer
}

func NewMailer(cfg Config) *SMTPMailer {
	return &SMTPMailer{cfg: cfg, dialer: DefaultDialer(cfg)}
}

// NewMailerWithDialer is the test seam: it lets callers (cicd tests) inject
// a Dialer, e.g. the in-memory mock from cicd/support.
func NewMailerWithDialer(cfg Config, dialer Dialer) *SMTPMailer {
	return &SMTPMailer{cfg: cfg, dialer: dialer}
}

func (m *SMTPMailer) IsConfigured() bool { return m.cfg.IsConfigured() }

func (m *SMTPMailer) Send(to []string, subject, body, htmlBody string) error {
	if !m.cfg.IsConfigured() {
		return errors.New("smtp not configured")
	}
	if len(to) == 0 {
		return errors.New("no recipients")
	}
	client, err := m.dialer.Dial()
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer client.Close()
	if err := client.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, r := range to {
		if err := client.Rcpt(r); err != nil {
			return fmt.Errorf("smtp rcpt to %s: %w", r, err)
		}
	}
	if err := client.Data(buildMessage(m.cfg.From, to, subject, body, htmlBody)); err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	return nil
}

// stdClient adapts *stdsmtp.Client to the Client interface. The stdlib's
// Data() returns (io.WriteCloser, error); this wrapper makes the whole
// message a single Data([]byte) call.
type stdClient struct {
	c *stdsmtp.Client
}

func (s *stdClient) Mail(from string) error { return s.c.Mail(from) }

func (s *stdClient) Rcpt(to string) error { return s.c.Rcpt(to) }

func (s *stdClient) Data(data []byte) error {
	w, err := s.c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func (s *stdClient) Close() error { return s.c.Close() }

type netDialer struct {
	cfg     Config
	timeout time.Duration
}

func DefaultDialer(cfg Config) Dialer {
	return &netDialer{cfg: cfg, timeout: 30 * time.Second}
}

func (d *netDialer) Dial() (Client, error) {
	addr := net.JoinHostPort(d.cfg.Host, strconv.Itoa(d.cfg.Port))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	// Bound the greeting/AUTH/STARTTLS handshake: a stalled relay must not
	// block the worker indefinitely.
	if d.timeout > 0 {
		conn.SetDeadline(time.Now().Add(d.timeout))
	}
	client, err := stdsmtp.NewClient(conn, d.cfg.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if d.cfg.StartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: d.cfg.Host}); err != nil {
			client.Close()
			return nil, err
		}
	}
	if d.cfg.Username != "" {
		auth := stdsmtp.PlainAuth("", d.cfg.Username, d.cfg.Password, d.cfg.Host)
		if err := client.Auth(auth); err != nil {
			client.Close()
			return nil, err
		}
	}
	return &stdClient{c: client}, nil
}

// buildMessage renders an RFC-5322 message with CRLF line endings (RFC 5321
// requires CRLF in the DATA payload). When htmlBody is non-empty the message
// is multipart/alternative with plain and html parts.
func buildMessage(from string, to []string, subject, body, htmlBody string) []byte {
	subject = strings.NewReplacer("\n", " ", "\r", " ").Replace(subject)
	toHeader := strings.Join(to, ", ")
	date := time.Now().UTC().Format(time.RFC1123)
	if htmlBody == "" {
		return []byte(fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s\r\n",
			from, toHeader, subject, date, body))
	}
	boundary := "automata-delivery-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%s\r\n\r\n",
		from, toHeader, subject, date, boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s\r\n", boundary, body)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=\"utf-8\"\r\n\r\n%s\r\n", boundary, htmlBody)
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}
