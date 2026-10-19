package captcha

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

type Config struct {
	Width             int
	Height            int
	ExpirationSeconds int64
}

func normalizeConfig(config Config) (Config, error) {
	if config.Width == 0 {
		config.Width = Width
	}
	if config.Height == 0 {
		config.Height = Height
	}
	if config.ExpirationSeconds == 0 {
		config.ExpirationSeconds = TTLSeconds
	}
	if config.Width < 120 || config.Width > 1024 {
		return Config{}, fmt.Errorf("captcha width must be between 120 and 1024 pixels")
	}
	if config.Height < 40 || config.Height > 512 {
		return Config{}, fmt.Errorf("captcha height must be between 40 and 512 pixels")
	}
	if config.ExpirationSeconds < 1 || config.ExpirationSeconds > 86400 {
		return Config{}, fmt.Errorf("captcha expiration must be between 1 and 86400 seconds")
	}
	return config, nil
}

func Generate(config Config) (id string, pngData []byte, err error) {
	config, err = normalizeConfig(config)
	if err != nil {
		return "", nil, err
	}
	var digits [Length]byte
	var answer [Length]byte
	for index := range digits {
		digit, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", nil, fmt.Errorf("generate captcha digits: %w", err)
		}
		digits[index] = byte(digit.Int64())
		answer[index] = '0' + digits[index]
	}
	pngData, err = renderPNG(digits, config.Width, config.Height)
	if err != nil {
		return "", nil, fmt.Errorf("render captcha: %w", err)
	}
	id, err = storeAnswer(string(answer[:]), config.ExpirationSeconds)
	if err != nil {
		return "", nil, err
	}
	return id, pngData, nil
}
