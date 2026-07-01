package crypto

import "golang.org/x/crypto/bcrypt"

// BcryptCost is fixed at 12 per the security spec.
const BcryptCost = 12

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// VerifyPassword reports whether password matches the bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
