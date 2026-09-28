package security

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/LazyEasyDev/LZApp/config/security_config"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrBcryptNotInitialized = errors.New("bcrypt password hasher is not initialized")
	ErrInvalidPasswordHash  = errors.New("invalid bcrypt password hash")
	bcryptHashPattern       = regexp.MustCompile(`^\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}$`)
)

const maxBcryptCost = 12

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(settings *security_config.SecurityBcryptConfig) (*BcryptHasher, error) {
	if settings == nil {
		return nil, fmt.Errorf("bcrypt configuration is required")
	}
	if settings.Cost < bcrypt.DefaultCost || settings.Cost > maxBcryptCost {
		return nil, fmt.Errorf("bcrypt cost must be between %d and %d", bcrypt.DefaultCost, maxBcryptCost)
	}
	return &BcryptHasher{cost: settings.Cost}, nil
}

func (hasher *BcryptHasher) HashPassword(password string) (string, error) {
	if hasher == nil || hasher.cost == 0 {
		return "", ErrBcryptNotInitialized
	}
	if len(password) > 72 {
		return "", bcrypt.ErrPasswordTooLong
	}
	encodedHash, err := bcrypt.GenerateFromPassword([]byte(password), hasher.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(encodedHash), nil
}

func (hasher *BcryptHasher) VerifyPassword(password, encodedHash string) (bool, error) {
	if hasher == nil || hasher.cost == 0 {
		return false, ErrBcryptNotInitialized
	}
	if len(password) > 72 {
		return false, bcrypt.ErrPasswordTooLong
	}
	if len(encodedHash) != 60 || !bcryptHashPattern.MatchString(encodedHash) {
		return false, ErrInvalidPasswordHash
	}
	hashBytes := []byte(encodedHash)
	cost, err := bcrypt.Cost(hashBytes)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidPasswordHash, err)
	}
	if cost > maxBcryptCost {
		return false, fmt.Errorf("%w: cost exceeds %d", ErrInvalidPasswordHash, maxBcryptCost)
	}
	if err := bcrypt.CompareHashAndPassword(hashBytes, []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, fmt.Errorf("%w: %w", ErrInvalidPasswordHash, err)
	}
	return true, nil
}
