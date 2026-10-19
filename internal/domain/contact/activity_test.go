package contact

import "testing"

func TestFormatActivityTitleEmailSent(t *testing.T) {
	a := Activity{
		Type: ActivityEmailSent,
		Data: map[string]interface{}{"subject": "Hi"},
	}
	if got := FormatActivityTitle(a); got != "Email sent: Hi" {
		t.Errorf("FormatActivityTitle = %q, want %q", got, "Email sent: Hi")
	}
}

func TestFormatActivityTitleEmailSentNoSubject(t *testing.T) {
	a := Activity{
		Type: ActivityEmailSent,
		Data: map[string]interface{}{},
	}
	if got := FormatActivityTitle(a); got != "Email sent" {
		t.Errorf("FormatActivityTitle = %q, want %q", got, "Email sent")
	}
}
