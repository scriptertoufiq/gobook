// Package hash wraps bcrypt so callers never pick a cost by hand.
package hash

import "golang.org/x/crypto/bcrypt"

func Make(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(bytes), err
}

func Check(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
