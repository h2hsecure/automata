package smtp

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// In-test fakes:
type fakeClient struct {
	calls   []string // recorded: "mail:addr", "rcpt:addr", "data:<len>", "close"
	mailErr error
	rcptErr error
	dataErr error
}

func (f *fakeClient) Mail(from string) error {
	f.calls = append(f.calls, "mail:"+from)
	return f.mailErr
}
func (f *fakeClient) Rcpt(to string) error { f.calls = append(f.calls, "rcpt:"+to); return f.rcptErr }
func (f *fakeClient) Data(data []byte) error {
	f.calls = append(f.calls, fmt.Sprintf("data:%d", len(data)))
	return f.dataErr
}
func (f *fakeClient) Close() error { f.calls = append(f.calls, "close"); return nil }

type fakeDialer struct {
	client Client
	err    error
	dialed bool
}

func (d *fakeDialer) Dial() (Client, error) { d.dialed = true; return d.client, d.err }

func TestBuildMessagePlain(t *testing.T) {
	msg := string(buildMessage("from@x.com", []string{"a@x.com"}, "Hello", "plain body", ""))
	for _, want := range []string{"Content-Type: text/plain", "From: from@x.com", "To: a@x.com", "Subject: Hello", "Date: ", "plain body"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "multipart") {
		t.Errorf("plain message must not contain multipart:\n%s", msg)
	}
}

func TestBuildMessageMultipart(t *testing.T) {
	msg := string(buildMessage("from@x.com", []string{"a@x.com"}, "Hello", "plain body", "<h1>Hi</h1>"))
	for _, want := range []string{"multipart/alternative", "boundary=", "Content-Type: text/plain", "Content-Type: text/html", "plain body", "<h1>Hi</h1>"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestBuildMessageSubjectSanitized(t *testing.T) {
	msg := string(buildMessage("from@x.com", []string{"a@x.com"}, "Hello\nInjected-Header: x", "body", ""))
	lines := strings.Split(msg, "\r\n")
	var subjectLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "Subject: ") {
			subjectLine = l
		}
	}
	if subjectLine != "Subject: Hello Injected-Header: x" {
		t.Errorf("subject line = %q, want %q", subjectLine, "Subject: Hello Injected-Header: x")
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "Injected-Header:") {
			t.Errorf("message contains injected header line %q", l)
		}
	}
}

func TestSendUnconfigured(t *testing.T) {
	d := &fakeDialer{}
	m := NewMailerWithDialer(Config{}, d)
	err := m.Send("ctx-1", []string{"a@x.com"}, "s", "b", "")
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected 'not configured' error, got %v", err)
	}
	if d.dialed {
		t.Error("dialer was called for unconfigured mailer")
	}
}

func TestSendNoRecipients(t *testing.T) {
	m := NewMailerWithDialer(Config{Host: "h", From: "f@x.com"}, &fakeDialer{})
	err := m.Send("ctx-1", nil, "s", "b", "")
	if err == nil || !strings.Contains(err.Error(), "no recipients") {
		t.Fatalf("expected 'no recipients' error, got %v", err)
	}
}

func TestSendHappyPath(t *testing.T) {
	fc := &fakeClient{}
	m := NewMailerWithDialer(Config{Host: "h", From: "from@x.com"}, &fakeDialer{client: fc})
	err := m.Send("ctx-1", []string{"a@x.com", "b@x.com"}, "s", "b", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"mail:from@x.com", "rcpt:a@x.com", "rcpt:b@x.com"}
	for i, w := range want {
		if i >= len(fc.calls) || fc.calls[i] != w {
			t.Fatalf("call %d = %q, want %q (all: %v)", i, fc.calls[i], w, fc.calls)
		}
	}
	if len(fc.calls) != 5 {
		t.Fatalf("expected exactly 5 calls, got %v", fc.calls)
	}
	if !strings.HasPrefix(fc.calls[3], "data:") || len(fc.calls[3]) <= len("data:") {
		t.Fatalf("call 3 = %q, want data:<len> with len > 0 (all: %v)", fc.calls[3], fc.calls)
	}
	if fc.calls[4] != "close" {
		t.Fatalf("call 4 = %q, want close (all: %v)", fc.calls[4], fc.calls)
	}
}

func TestSendRcptFailure(t *testing.T) {
	fc := &fakeClient{rcptErr: errors.New("rcpt failed")}
	m := NewMailerWithDialer(Config{Host: "h", From: "from@x.com"}, &fakeDialer{client: fc})
	err := m.Send("ctx-1", []string{"a@x.com"}, "s", "b", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	sawMail, sawData, sawClose := false, false, false
	for _, c := range fc.calls {
		switch {
		case strings.HasPrefix(c, "mail:"):
			sawMail = true
		case strings.HasPrefix(c, "data:"):
			sawData = true
		case c == "close":
			sawClose = true
		}
	}
	if !sawMail {
		t.Error("mail was not called")
	}
	if sawData {
		t.Error("data was called after rcpt failure")
	}
	if !sawClose {
		t.Error("close was not called (deferred)")
	}
}

func TestSendDataFailure(t *testing.T) {
	fc := &fakeClient{dataErr: errors.New("data failed")}
	m := NewMailerWithDialer(Config{Host: "h", From: "from@x.com"}, &fakeDialer{client: fc})
	err := m.Send("ctx-1", []string{"a@x.com"}, "s", "b", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	sawClose := false
	for _, c := range fc.calls {
		if c == "close" {
			sawClose = true
		}
	}
	if !sawClose {
		t.Error("close was not called (deferred)")
	}
}

func TestSendDialFailure(t *testing.T) {
	m := NewMailerWithDialer(Config{Host: "h", From: "f@x.com"}, &fakeDialer{err: errors.New("dial failed")})
	err := m.Send("ctx-1", []string{"a@x.com"}, "s", "b", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestNetDialerTimesOutOnStalledGreeting verifies the post-connect deadline:
// a server that accepts the TCP connection but never sends the SMTP greeting
// must not block Dial forever.
func TestNetDialerTimesOutOnStalledGreeting(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		// Accepted but never sends the greeting: stall until the test ends.
		defer conn.Close()
		time.Sleep(3 * time.Second)
	}()
	d := &netDialer{cfg: Config{Host: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port}, timeout: 200 * time.Millisecond}
	start := time.Now()
	_, err = d.Dial()
	if err == nil {
		t.Fatal("expected dial error against stalled server, got nil")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("dial took too long (deadline not enforced): %v", elapsed)
	}
}
