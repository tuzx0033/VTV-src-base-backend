// Package xcrypto holds password hashing (bcrypt).
package xcrypto

import "golang.org/x/crypto/bcrypt"

// Cost is the bcrypt cost factor (matches the legacy system → old hashes verify).
const Cost = 12

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), Cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ComparePassword reports whether plain matches the bcrypt hash.
func ComparePassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
