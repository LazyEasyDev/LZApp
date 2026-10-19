package users

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", errors.New("invalid email address")
	}
	return email, nil
}

func ValidateNewPassword(password string) error {
	if utf8.RuneCountInString(password) < 8 || len(password) > 72 {
		return errors.New("password must contain at least 8 characters and at most 72 bytes")
	}
	return nil
}
