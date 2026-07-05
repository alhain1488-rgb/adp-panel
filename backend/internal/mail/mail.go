// Package mail sends e-mail over SMTP using operator-configured credentials
// (stored encrypted at rest, like every other secret). It backs two features:
// e-mail delivery of encrypted backups and sending a client their subscription.
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/store"
)

// Settings keys (KV). The SMTP password is encrypted with the panel master key.
const (
	keyEnabled  = "smtp_enabled"
	keyHost     = "smtp_host"
	keyPort     = "smtp_port"
	keyUser     = "smtp_user"
	keyPassEnc  = "smtp_pass_enc"
	keyFrom     = "smtp_from"
	keySecurity = "smtp_security" // "starttls" | "tls" | "none"
)

// SecurityModes are the accepted transport-security values.
var SecurityModes = map[string]bool{"starttls": true, "tls": true, "none": true}

// Status is the safe, secret-free view returned to the UI.
type Status struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	HasPassword bool   `json:"has_password"`
	From        string `json:"from"`
	Security    string `json:"security"`
}

// Input updates the config. A blank Password means "keep the stored one".
type Input struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	Security string `json:"security"`
}

// Mailer sends mail using the persisted SMTP config.
type Mailer struct {
	store       *store.Store
	cipher      *crypto.Cipher
	dialTimeout time.Duration
}

// NewMailer builds a Mailer.
func NewMailer(st *store.Store, cipher *crypto.Cipher) *Mailer {
	return &Mailer{store: st, cipher: cipher, dialTimeout: 20 * time.Second}
}

// Status returns the config without exposing the password.
func (m *Mailer) Status(ctx context.Context) (Status, error) {
	get := func(k string) (string, error) { return m.store.GetSetting(ctx, k) }
	enabled, err := get(keyEnabled)
	if err != nil {
		return Status{}, err
	}
	host, err := get(keyHost)
	if err != nil {
		return Status{}, err
	}
	port, err := get(keyPort)
	if err != nil {
		return Status{}, err
	}
	user, err := get(keyUser)
	if err != nil {
		return Status{}, err
	}
	passEnc, err := get(keyPassEnc)
	if err != nil {
		return Status{}, err
	}
	from, err := get(keyFrom)
	if err != nil {
		return Status{}, err
	}
	security, err := get(keySecurity)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Enabled:     enabled == "1",
		Host:        host,
		Port:        parsePort(port),
		Username:    user,
		HasPassword: passEnc != "",
		From:        from,
		Security:    normalizeSecurity(security),
	}, nil
}

// SetConfig persists the config, encrypting the password. A blank password is
// left untouched so the UI needn't resend it.
func (m *Mailer) SetConfig(ctx context.Context, in Input) error {
	set := func(k, v string) error { return m.store.SetSetting(ctx, k, v) }
	if in.Enabled {
		if err := set(keyEnabled, "1"); err != nil {
			return err
		}
	} else if err := set(keyEnabled, ""); err != nil {
		return err
	}
	if err := set(keyHost, strings.TrimSpace(in.Host)); err != nil {
		return err
	}
	if err := set(keyPort, strconv.Itoa(in.Port)); err != nil {
		return err
	}
	if err := set(keyUser, strings.TrimSpace(in.Username)); err != nil {
		return err
	}
	if err := set(keyFrom, strings.TrimSpace(in.From)); err != nil {
		return err
	}
	if err := set(keySecurity, normalizeSecurity(in.Security)); err != nil {
		return err
	}
	if in.Password != "" {
		enc, err := m.cipher.Encrypt(in.Password)
		if err != nil {
			return err
		}
		if err := set(keyPassEnc, enc); err != nil {
			return err
		}
	}
	return nil
}

// Configured reports whether mail can be sent (enabled + host + from present).
func (m *Mailer) Configured(ctx context.Context) bool {
	st, err := m.Status(ctx)
	if err != nil {
		return false
	}
	return st.Enabled && st.Host != "" && st.From != ""
}

// resolved is the SMTP config with the password decrypted.
type resolved struct {
	host, from, user, pass, security string
	port                             int
	enabled                          bool
}

func (m *Mailer) config(ctx context.Context) (resolved, error) {
	st, err := m.Status(ctx)
	if err != nil {
		return resolved{}, err
	}
	if !st.Enabled {
		return resolved{}, fmt.Errorf("mail: SMTP is not enabled")
	}
	if st.Host == "" || st.From == "" {
		return resolved{}, fmt.Errorf("mail: set the SMTP host and From address first")
	}
	r := resolved{
		host: st.Host, from: st.From, user: st.Username,
		port: st.Port, security: st.Security, enabled: true,
	}
	if r.port == 0 {
		r.port = defaultPort(r.security)
	}
	passEnc, err := m.store.GetSetting(ctx, keyPassEnc)
	if err != nil {
		return resolved{}, err
	}
	if passEnc != "" {
		if r.pass, err = m.cipher.Decrypt(passEnc); err != nil {
			return resolved{}, err
		}
	}
	return r, nil
}

// Attachment is an optional file carried by a message.
type Attachment struct {
	Filename    string
	Data        []byte
	ContentType string // defaults to application/octet-stream
}

// Send delivers a plain-text message (with an optional attachment) to recipients.
func (m *Mailer) Send(ctx context.Context, to []string, subject, body string, att *Attachment) error {
	cfg, err := m.config(ctx)
	if err != nil {
		return err
	}
	recipients := make([]string, 0, len(to))
	for _, t := range to {
		if t = strings.TrimSpace(t); t != "" {
			recipients = append(recipients, t)
		}
	}
	if len(recipients) == 0 {
		return fmt.Errorf("mail: no recipient")
	}

	msg := buildMessage(cfg.from, recipients, subject, body, att)
	return m.deliver(ctx, cfg, recipients, msg)
}

// deliver opens the SMTP connection (implicit TLS, STARTTLS or plain), performs
// auth if a username is set, and sends the message.
func (m *Mailer) deliver(ctx context.Context, cfg resolved, to []string, msg []byte) error {
	addr := net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port))
	d := net.Dialer{Timeout: m.dialTimeout}

	var client *smtp.Client
	var err error
	if cfg.security == "tls" {
		conn, derr := tls.DialWithDialer(&d, "tcp", addr, &tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12})
		if derr != nil {
			return fmt.Errorf("mail: connect: %w", derr)
		}
		client, err = smtp.NewClient(conn, cfg.host)
	} else {
		conn, derr := d.DialContext(ctx, "tcp", addr)
		if derr != nil {
			return fmt.Errorf("mail: connect: %w", derr)
		}
		client, err = smtp.NewClient(conn, cfg.host)
	}
	if err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	defer func() { _ = client.Close() }()

	if cfg.security == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("mail: server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}
	if cfg.user != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.user, cfg.pass, cfg.host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := client.Mail(cfg.from); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("mail: rcpt %q: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	return client.Quit()
}

// buildMessage renders an RFC 5322 message: a text/plain body, optionally wrapped
// in multipart/mixed with a base64 attachment.
func buildMessage(from string, to []string, subject, body string, att *Attachment) []byte {
	var b bytes.Buffer
	writeHeader := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	writeHeader("From", from)
	writeHeader("To", strings.Join(to, ", "))
	writeHeader("Subject", mime.QEncoding.Encode("utf-8", subject))
	writeHeader("MIME-Version", "1.0")

	if att == nil {
		writeHeader("Content-Type", "text/plain; charset=utf-8")
		b.WriteString("\r\n")
		b.WriteString(normalizeCRLF(body))
		return b.Bytes()
	}

	mw := multipart.NewWriter(&b)
	writeHeader("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	b.WriteString("\r\n")

	textPart, _ := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	_, _ = textPart.Write([]byte(normalizeCRLF(body)))

	ct := att.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	filePart, _ := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {ct},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {fmt.Sprintf("attachment; filename=%q", att.Filename)},
	})
	_, _ = filePart.Write(base64Wrap(att.Data))
	_ = mw.Close()
	return b.Bytes()
}

// base64Wrap standard-base64-encodes data, wrapped at 76 columns per MIME.
func base64Wrap(data []byte) []byte {
	enc := base64.StdEncoding.EncodeToString(data)
	var out bytes.Buffer
	for len(enc) > 76 {
		out.WriteString(enc[:76])
		out.WriteString("\r\n")
		enc = enc[76:]
	}
	out.WriteString(enc)
	return out.Bytes()
}

func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func normalizeSecurity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !SecurityModes[s] {
		return "starttls"
	}
	return s
}

func defaultPort(security string) int {
	if security == "tls" {
		return 465
	}
	return 587
}

func parsePort(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
