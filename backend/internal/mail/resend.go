package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// resendEndpoint is the Resend "send e-mail" API. It is a package var so tests
// can point it at a stub server.
var resendEndpoint = "https://api.resend.com/emails"

// sendResend delivers a message through Resend's HTTPS API (port 443), which
// works on hosts whose provider blocks outbound SMTP.
func (m *Mailer) sendResend(ctx context.Context, cfg resolved, to []string, subject, body string, att *Attachment) error {
	type attachment struct {
		Filename string `json:"filename"`
		Content  string `json:"content"` // base64
	}
	payload := map[string]any{
		"from":    cfg.from,
		"to":      to,
		"subject": subject,
		"text":    body,
	}
	if att != nil {
		payload["attachments"] = []attachment{{
			Filename: att.Filename,
			Content:  base64.StdEncoding.EncodeToString(att.Data),
		}}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.resendKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mail: resend request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// Resend returns {"message":"...","name":"..."} on error.
	var e struct {
		Message string `json:"message"`
		Name    string `json:"name"`
	}
	_ = json.Unmarshal(respBody, &e)
	if e.Message != "" {
		return fmt.Errorf("mail: resend: %s", e.Message)
	}
	return fmt.Errorf("mail: resend request failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
}
