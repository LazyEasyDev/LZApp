package http_config

import "fmt"

type HTTPConfig struct {
	APITitle           string `json:"api_title"`
	APIVersion         string `json:"api_version"`
	APITokenCookieName string `json:"api_token_cookie_name"`
	// CookieMaxAgeSeconds applies to both login and referral cookies.
	CookieMaxAgeSeconds      int    `json:"cookie_max_age_seconds"`
	HTTPSPort                int    `json:"https_port"`
	HTTPSCertificate         string `json:"https_certificate"`
	HTTPSKey                 string `json:"https_key"`
	ReadHeaderTimeoutSeconds int    `json:"read_header_timeout_seconds"`
	IdleTimeoutSeconds       int    `json:"idle_timeout_seconds"`
	ReadTimeoutSeconds       int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds      int    `json:"write_timeout_seconds"`
	ShutdownTimeoutSeconds   int    `json:"shutdown_timeout_seconds"`
}

func (config *HTTPConfig) ValidateCookieMaxAge() error {
	if config == nil || config.CookieMaxAgeSeconds <= 0 || config.CookieMaxAgeSeconds > 400*24*60*60 {
		return fmt.Errorf("cookie_max_age_seconds must be between 1 and %d", 400*24*60*60)
	}
	return nil
}
