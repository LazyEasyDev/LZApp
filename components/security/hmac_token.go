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

	"github.com/LazyEasyDev/LZApp/config/security_config"
)

var (
	ErrInvalidHMACToken = errors.New("invalid HMAC token")
	ErrNotInitialized   = errors.New("HMAC signer is not initialized")
)

const maxHMACTokenLength = 4096

type HMACTokenOptions struct {
	PayloadBytes   int
	SignatureBytes int
}

type HMACTokenSigner struct {
	key            []byte
	payloadBytes   int
	signatureBytes int
}

func NewHMACTokenSigner(hmacConfig *security_config.SecurityHMACConfig) (*HMACTokenSigner, error) {
	if hmacConfig == nil {
		return nil, fmt.Errorf("HMAC configuration is required")
	}

	key := []byte(hmacConfig.HMACKey)

	PayloadBytes := hmacConfig.HMACTokenBytes
	if PayloadBytes == 0 {
		PayloadBytes = sha256.Size
	}
	SignatureBytes := hmacConfig.HMACTokenBytes
	if SignatureBytes == 0 {
		SignatureBytes = sha256.Size
	}

	for _, size := range []struct {
		name  string
		bytes int
	}{
		{name: "payload", bytes: PayloadBytes},
		{name: "signature", bytes: SignatureBytes},
	} {
		if size.bytes < 16 || size.bytes > sha256.Size {
			return nil, fmt.Errorf("HMAC token %s size must be between 16 and %d bytes", size.name, sha256.Size)
		}
	}
	signer := &HMACTokenSigner{
		payloadBytes:   PayloadBytes,
		signatureBytes: SignatureBytes,
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

func (signer *HMACTokenSigner) GenerateRandToken() (string, error) {
	if signer == nil || len(signer.key) < sha256.Size {
		return "", ErrNotInitialized
	}
	payload := make([]byte, signer.payloadBytes)
	if _, err := rand.Read(payload); err != nil {
		return "", fmt.Errorf("generate token payload: %w", err)
	}
	return signer.GenerateToken([]string{string(payload)})
}

func (signer *HMACTokenSigner) GenerateToken(fields []string) (string, error) {
	if signer == nil || len(signer.key) < sha256.Size {
		return "", ErrNotInitialized
	}
	if len(fields) == 0 {
		return "", fmt.Errorf("at least one token field is required")
	}
	tokenLength := 2 + base64.RawURLEncoding.EncodedLen(signer.signatureBytes) + len(fields)
	if tokenLength > maxHMACTokenLength {
		return "", fmt.Errorf("token exceeds %d characters", maxHMACTokenLength)
	}
	encodedFields := make([]string, len(fields))
	for index, field := range fields {
		if len(field) > maxHMACTokenLength {
			return "", fmt.Errorf("token exceeds %d characters", maxHMACTokenLength)
		}
		tokenLength += base64.RawURLEncoding.EncodedLen(len(field))
		if tokenLength > maxHMACTokenLength {
			return "", fmt.Errorf("token exceeds %d characters", maxHMACTokenLength)
		}
		encodedFields[index] = base64.RawURLEncoding.EncodeToString([]byte(field))
	}
	signedPart := strings.Join(encodedFields, ".")
	return "A" + signedPart + "." +
		base64.RawURLEncoding.EncodeToString(signer.signature([]byte(signedPart))) + "Z", nil
}

func (signer *HMACTokenSigner) VerifyToken(token string) ([]string, error) {
	if signer == nil || len(signer.key) < sha256.Size {
		return nil, ErrNotInitialized
	}
	signatureSize := base64.RawURLEncoding.EncodedLen(signer.signatureBytes)
	if len(token) < signatureSize+3 || len(token) > maxHMACTokenLength || token[0] != 'A' || token[len(token)-1] != 'Z' {
		return nil, ErrInvalidHMACToken
	}
	inner := token[1 : len(token)-1]
	separator := strings.LastIndexByte(inner, '.')
	if separator < 0 {
		return nil, ErrInvalidHMACToken
	}
	signedPart, signatureText := inner[:separator], inner[separator+1:]
	if len(signatureText) != signatureSize {
		return nil, ErrInvalidHMACToken
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(signatureText)
	if err != nil || len(signature) != signer.signatureBytes || base64.RawURLEncoding.EncodeToString(signature) != signatureText {
		return nil, ErrInvalidHMACToken
	}
	if !hmac.Equal(signature, signer.signature([]byte(signedPart))) {
		return nil, ErrInvalidHMACToken
	}
	fields := strings.Split(signedPart, ".")
	for index, encoded := range fields {
		decoded, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
		if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
			return nil, ErrInvalidHMACToken
		}
		fields[index] = string(decoded)
	}
	return fields, nil
}

func (signer *HMACTokenSigner) signature(payload []byte) []byte {
	mac := hmac.New(sha256.New, signer.key)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)[:signer.signatureBytes]
}
