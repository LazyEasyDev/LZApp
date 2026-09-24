package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidHMACToken = errors.New("invalid HMAC token")
	ErrNotInitialized   = errors.New("HMAC signer is not initialized")
)

type HMACTokenOptions struct {
	PayloadBytes   int
	SignatureBytes int
}

type HMACTokenSigner struct {
	key            []byte
	payloadBytes   int
	signatureBytes int
}

func NewHMACTokenSigner(key []byte, options ...HMACTokenOptions) (*HMACTokenSigner, error) {
	if len(options) > 1 {
		return nil, fmt.Errorf("at most one HMAC token options value is allowed")
	}
	var settings HMACTokenOptions
	if len(options) == 1 {
		settings = options[0]
	}
	if settings.PayloadBytes == 0 {
		settings.PayloadBytes = sha256.Size
	}
	if settings.SignatureBytes == 0 {
		settings.SignatureBytes = sha256.Size
	}
	for _, size := range []struct {
		name  string
		bytes int
	}{
		{name: "payload", bytes: settings.PayloadBytes},
		{name: "signature", bytes: settings.SignatureBytes},
	} {
		if size.bytes < 16 || size.bytes > sha256.Size {
			return nil, fmt.Errorf("HMAC token %s size must be between 16 and %d bytes", size.name, sha256.Size)
		}
	}
	signer := &HMACTokenSigner{
		payloadBytes:   settings.PayloadBytes,
		signatureBytes: settings.SignatureBytes,
	}
	key = bytes.TrimSpace(key)
	if len(key) == 0 {
		signer.key = make([]byte, sha256.Size)
		if _, err := rand.Read(signer.key); err != nil {
			return nil, fmt.Errorf("generate HMAC signing key: %w", err)
		}
	} else {
		derivedKey := sha256.Sum256(key)
		signer.key = derivedKey[:]
	}
	return signer, nil
}

func (signer *HMACTokenSigner) Generate() (string, error) {
	if signer == nil || len(signer.key) < sha256.Size {
		return "", ErrNotInitialized
	}
	payload := make([]byte, signer.payloadBytes)
	if _, err := rand.Read(payload); err != nil {
		return "", fmt.Errorf("generate token payload: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(signer.signature(payload)), nil
}

func (signer *HMACTokenSigner) Verify(token string) error {
	if signer == nil || len(signer.key) < sha256.Size {
		return ErrNotInitialized
	}
	payloadSize := base64.RawURLEncoding.EncodedLen(signer.payloadBytes)
	signatureSize := base64.RawURLEncoding.EncodedLen(signer.signatureBytes)
	if len(token) != payloadSize+signatureSize+1 {
		return ErrInvalidHMACToken
	}
	payloadText, signatureText, found := strings.Cut(token, ".")
	if !found || len(payloadText) != payloadSize || len(signatureText) != signatureSize {
		return ErrInvalidHMACToken
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(payloadText)
	if err != nil || len(payload) != signer.payloadBytes || base64.RawURLEncoding.EncodeToString(payload) != payloadText {
		return ErrInvalidHMACToken
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(signatureText)
	if err != nil || len(signature) != signer.signatureBytes || base64.RawURLEncoding.EncodeToString(signature) != signatureText {
		return ErrInvalidHMACToken
	}
	if !hmac.Equal(signature, signer.signature(payload)) {
		return ErrInvalidHMACToken
	}
	return nil
}

func (signer *HMACTokenSigner) signature(payload []byte) []byte {
	mac := hmac.New(sha256.New, signer.key)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)[:signer.signatureBytes]
}
