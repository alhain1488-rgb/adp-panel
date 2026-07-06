package mail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	att := Attachment{Filename: "backup.adpbak", Data: []byte("SECRETBYTES"), ContentType: "application/octet-stream"}
	msg := string(buildMessage("panel@example.com", []string{"a@example.com"}, "Backup", "see attached", []Attachment{att}))
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

func TestSendResend(t *testing.T) {
	var gotAuth, gotCT string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	old := resendEndpoint
	resendEndpoint = srv.URL
	defer func() { resendEndpoint = old }()

	m := &Mailer{httpClient: srv.Client()}
	cfg := resolved{provider: "resend", from: "panel@example.com", resendKey: "re_test123"}
	att := Attachment{Filename: "b.adpbak", Data: []byte("bytes")}
	if err := m.sendResend(context.Background(), cfg, []string{"a@example.com"}, "Subj", "Body", []Attachment{att}); err != nil {
		t.Fatalf("sendResend: %v", err)
	}
	if gotAuth != "Bearer re_test123" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q", gotCT)
	}
	if gotBody["from"] != "panel@example.com" || gotBody["subject"] != "Subj" || gotBody["text"] != "Body" {
		t.Errorf("body = %+v", gotBody)
	}
	if _, ok := gotBody["attachments"]; !ok {
		t.Error("attachment not sent")
	}
}

func TestSendResend_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"domain is not verified","name":"validation_error"}`))
	}))
	defer srv.Close()
	old := resendEndpoint
	resendEndpoint = srv.URL
	defer func() { resendEndpoint = old }()

	m := &Mailer{httpClient: srv.Client()}
	cfg := resolved{provider: "resend", from: "x@x.com", resendKey: "re_x"}
	err := m.sendResend(context.Background(), cfg, []string{"a@b.com"}, "s", "b", nil)
	if err == nil || !strings.Contains(err.Error(), "domain is not verified") {
		t.Fatalf("expected API error surfaced, got %v", err)
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
