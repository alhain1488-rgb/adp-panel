package backup

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/adp/panel/internal/mail"
	"github.com/adp/panel/internal/store"
)

// Settings keys (KV). SMTP credentials live in the mail package; here we only
// keep the backup-specific recipient, passphrase and schedule.
const (
	keyEmEnabled  = "backup_email_enabled"
	keyEmTo       = "backup_email_to"
	keyEmPassEnc  = "backup_email_passphrase_enc"
	keyEmInterval = "backup_email_interval_hours"
	keyEmLastAt   = "backup_email_last_at"
	keyEmLastErr  = "backup_email_last_error"
	keyEmLastOK   = "backup_email_last_ok"
)

// EmailStatus is the secret-free view returned to the UI.
type EmailStatus struct {
	Enabled       bool   `json:"enabled"`
	To            string `json:"to"`
	HasPassphrase bool   `json:"has_passphrase"`
	IntervalHours int    `json:"interval_hours"`
	SMTPReady     bool   `json:"smtp_ready"`
	LastAt        string `json:"last_at"`
	LastError     string `json:"last_error"`
	LastOK        bool   `json:"last_ok"`
}

// EmailInput updates the config. A blank Passphrase keeps the stored one.
type EmailInput struct {
	Enabled       bool   `json:"enabled"`
	To            string `json:"to"`
	Passphrase    string `json:"passphrase"`
	IntervalHours int    `json:"interval_hours"`
}

// EmailCipher decrypts the stored backup passphrase (the crypto.Cipher).
type EmailCipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// Email schedules and sends encrypted backups to an e-mail address over SMTP.
type Email struct {
	svc    *Service
	mailer *mail.Mailer
	store  *store.Store
	cipher EmailCipher
	logger *slog.Logger
	now    func() time.Time
}

// NewEmail wires e-mail backup delivery.
func NewEmail(svc *Service, mailer *mail.Mailer, st *store.Store, cipher EmailCipher, logger *slog.Logger) *Email {
	return &Email{svc: svc, mailer: mailer, store: st, cipher: cipher, logger: logger, now: time.Now}
}

// Status returns the config without exposing the passphrase.
func (e *Email) Status(ctx context.Context) (EmailStatus, error) {
	get := func(k string) (string, error) { return e.store.GetSetting(ctx, k) }
	enabled, err := get(keyEmEnabled)
	if err != nil {
		return EmailStatus{}, err
	}
	to, err := get(keyEmTo)
	if err != nil {
		return EmailStatus{}, err
	}
	passEnc, err := get(keyEmPassEnc)
	if err != nil {
		return EmailStatus{}, err
	}
	interval, err := get(keyEmInterval)
	if err != nil {
		return EmailStatus{}, err
	}
	lastAt, err := get(keyEmLastAt)
	if err != nil {
		return EmailStatus{}, err
	}
	lastErr, err := get(keyEmLastErr)
	if err != nil {
		return EmailStatus{}, err
	}
	lastOK, err := get(keyEmLastOK)
	if err != nil {
		return EmailStatus{}, err
	}
	return EmailStatus{
		Enabled:       enabled == "1",
		To:            to,
		HasPassphrase: passEnc != "",
		IntervalHours: parseInterval(interval),
		SMTPReady:     e.mailer.Configured(ctx),
		LastAt:        lastAt,
		LastError:     lastErr,
		LastOK:        lastOK == "1",
	}, nil
}

// SetConfig persists the config, encrypting the passphrase.
func (e *Email) SetConfig(ctx context.Context, in EmailInput) error {
	set := func(k, v string) error { return e.store.SetSetting(ctx, k, v) }
	if in.Enabled {
		if err := set(keyEmEnabled, "1"); err != nil {
			return err
		}
	} else if err := set(keyEmEnabled, ""); err != nil {
		return err
	}
	if err := set(keyEmTo, in.To); err != nil {
		return err
	}
	interval := in.IntervalHours
	if interval < minIntervalHours {
		interval = defaultIntervalHours
	}
	if err := set(keyEmInterval, strconv.Itoa(interval)); err != nil {
		return err
	}
	if in.Passphrase != "" {
		enc, err := e.cipher.Encrypt(in.Passphrase)
		if err != nil {
			return err
		}
		if err := set(keyEmPassEnc, enc); err != nil {
			return err
		}
	}
	return nil
}

// RunNow builds a backup and e-mails it immediately, recording the outcome.
func (e *Email) RunNow(ctx context.Context) error {
	to, passphrase, err := e.credentials(ctx)
	if err != nil {
		return err
	}
	data, filename, err := e.svc.Export(ctx, passphrase)
	if err != nil {
		e.recordResult(ctx, err)
		return err
	}
	stamp := e.now().UTC().Format("2006-01-02 15:04 UTC")
	subject := "ADP panel backup — " + stamp
	body := fmt.Sprintf(
		"Encrypted ADP panel backup attached (%s).\n\n"+
			"Restore it from Settings → Backup → Import, using the passphrase you set for e-mail backups.\n",
		stamp)
	att := mail.Attachment{Filename: filename, Data: data, ContentType: "application/octet-stream"}
	if err := e.mailer.Send(ctx, []string{to}, subject, body, att); err != nil {
		e.recordResult(ctx, err)
		return err
	}
	e.recordResult(ctx, nil)
	return nil
}

// RunScheduler sends a backup whenever the configured interval has elapsed.
func (e *Email) RunScheduler(ctx context.Context) {
	ticker := time.NewTicker(schedulerTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e.due(ctx) {
				if err := e.RunNow(ctx); err != nil {
					e.logger.Warn("scheduled email backup failed", "err", err)
				} else {
					e.logger.Info("scheduled email backup sent")
				}
			}
		}
	}
}

func (e *Email) due(ctx context.Context) bool {
	st, err := e.Status(ctx)
	if err != nil || !st.Enabled || !st.HasPassphrase || st.To == "" || !st.SMTPReady {
		return false
	}
	if st.LastAt == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, st.LastAt)
	if err != nil {
		return true
	}
	return e.now().Sub(last) >= time.Duration(st.IntervalHours)*time.Hour
}

func (e *Email) credentials(ctx context.Context) (to, passphrase string, err error) {
	to, err = e.store.GetSetting(ctx, keyEmTo)
	if err != nil {
		return "", "", err
	}
	passEnc, err := e.store.GetSetting(ctx, keyEmPassEnc)
	if err != nil {
		return "", "", err
	}
	if to == "" || passEnc == "" {
		return "", "", fmt.Errorf("email backup: set a recipient and passphrase first")
	}
	if passphrase, err = e.cipher.Decrypt(passEnc); err != nil {
		return "", "", err
	}
	return to, passphrase, nil
}

func (e *Email) recordResult(ctx context.Context, runErr error) {
	_ = e.store.SetSetting(ctx, keyEmLastAt, e.now().UTC().Format(time.RFC3339))
	if runErr != nil {
		_ = e.store.SetSetting(ctx, keyEmLastOK, "")
		_ = e.store.SetSetting(ctx, keyEmLastErr, runErr.Error())
		return
	}
	_ = e.store.SetSetting(ctx, keyEmLastOK, "1")
	_ = e.store.SetSetting(ctx, keyEmLastErr, "")
}
