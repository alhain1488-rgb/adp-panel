package mail

import (
	"strings"
	"testing"
)

func TestBuildMessage_PlainText(t *testing.T) {
	msg := string(buildMessage("panel@example.com", []string{"a@example.com", "b@example.com"},
		"Hello", "line1\nline2", nil))
	for _, want := range []string{
		"From: panel@example.com\r\n",
		"To: a@example.com, b@example.com\r\n",
		"Subject: Hello\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"line1\r\nline2", // LF normalized to CRLF
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q\n---\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "multipart") {
		t.Error("no attachment should mean no multipart wrapper")
	}
}

func TestBuildMessage_WithAttachment(t *testing.T) {
	att := &Attachment{Filename: "backup.adpbak", Data: []byte("SECRETBYTES"), ContentType: "application/octet-stream"}
	msg := string(buildMessage("panel@example.com", []string{"a@example.com"}, "Backup", "see attached", att))
	for _, want := range []string{
		"Content-Type: multipart/mixed; boundary=",
		"Content-Type: text/plain; charset=utf-8",
		`Content-Disposition: attachment; filename="backup.adpbak"`,
		"Content-Transfer-Encoding: base64",
		"see attached",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q", want)
		}
	}
	// The raw secret bytes must be base64-encoded, not present verbatim.
	if strings.Contains(msg, "SECRETBYTES") {
		t.Error("attachment payload should be base64-encoded, not raw")
	}
}

func TestNormalizeSecurityAndPort(t *testing.T) {
	if normalizeSecurity("TLS") != "tls" {
		t.Error("TLS should normalize to tls")
	}
	if normalizeSecurity("bogus") != "starttls" {
		t.Error("unknown security should default to starttls")
	}
	if defaultPort("tls") != 465 {
		t.Error("implicit-TLS default port should be 465")
	}
	if defaultPort("starttls") != 587 {
		t.Error("starttls default port should be 587")
	}
}
