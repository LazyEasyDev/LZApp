package captcha

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/LazyEasyDev/LCache"
)

const (
	Length     = 6
	Width      = 240
	Height     = 80
	TTLSeconds = 120
	keyPrefix  = "captcha:"
)

var ErrCacheUnavailable = errors.New("captcha cache is unavailable")

type challenge struct {
	answer    string
	expiresAt time.Time
}

func storeAnswer(answer string, expirationSeconds int64) (string, error) {
	id := rand.Text()
	key := keyPrefix + id
	entry := &challenge{answer: answer, expiresAt: time.Now().Add(time.Duration(expirationSeconds) * time.Second)}
	LCache.Set(key, entry, expirationSeconds)
	if stored, found := LCache.Get(key); !found || stored != entry {
		return "", ErrCacheUnavailable
	}
	return id, nil
}

func Verify(id, answer string) bool {
	if len(id) != 26 {
		return false
	}
	key := keyPrefix + id
	value, found := LCache.Get(key)
	if !found || !LCache.Delete(key) {
		return false
	}
	entry, ok := value.(*challenge)
	if !ok || entry == nil || !time.Now().Before(entry.expiresAt) {
		return false
	}
	return len(answer) == Length && subtle.ConstantTimeCompare([]byte(entry.answer), []byte(answer)) == 1
}
