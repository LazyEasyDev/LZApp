package captcha

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/LazyEasyDev/LZApp/components"
)

const (
	Length     = 6
	Width      = 240
	Height     = 80
	TTLSeconds = 120
	keyPrefix  = "captcha:"
)

var ErrCacheUnavailable = errors.New("captcha cache is unavailable")

func storeAnswer(answer string, expirationSeconds int64) (string, error) {
	client := components.GetRedis()
	if client == nil {
		return "", ErrCacheUnavailable
	}
	id := rand.Text()
	key := keyPrefix + id
	if err := client.Set(context.Background(), key, answer, time.Duration(expirationSeconds)*time.Second).Err(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrCacheUnavailable, err)
	}
	return id, nil
}

func Verify(id, answer string) bool {
	if len(id) != 26 {
		return false
	}
	client := components.GetRedis()
	if client == nil {
		return false
	}
	key := keyPrefix + id
	storedAnswer, err := client.GetDel(context.Background(), key).Result()
	if err != nil {
		return false
	}
	return len(answer) == Length && subtle.ConstantTimeCompare([]byte(storedAnswer), []byte(answer)) == 1
}
