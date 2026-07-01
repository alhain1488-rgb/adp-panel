package auth

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
)

// totpIssuer is shown in authenticator apps.
const totpIssuer = "Absolutely Disgusting Panel"

// generateTOTP creates a new TOTP secret for the account and returns the secret,
// the otpauth:// URL, and a PNG data-URL QR code.
func generateTOTP(account string) (secret, otpauthURL, qrDataURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: totpIssuer, AccountName: account})
	if err != nil {
		return "", "", "", err
	}
	png, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		return "", "", "", err
	}
	var buf bytes.Buffer
	buf.WriteString("data:image/png;base64,")
	buf.WriteString(base64.StdEncoding.EncodeToString(png))
	return key.Secret(), key.URL(), buf.String(), nil
}

// validateTOTP reports whether code is currently valid for secret.
func validateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// otpauthURLFor rebuilds the provisioning URL for an existing secret (used if we
// need to re-render a QR without regenerating the secret).
func otpauthURLFor(account, secret string) (string, error) {
	key, err := otp.NewKeyFromURL(fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s", totpIssuer, account, secret, totpIssuer))
	if err != nil {
		return "", err
	}
	return key.URL(), nil
}
