package http_config

type HTTPConfig struct {
	APITokenCookieName       string `json:"api_token_cookie_name"`
	HTTPSPort                int    `json:"https_port"`
	HTTPSCertificate         string `json:"https_certificate"`
	HTTPSKey                 string `json:"https_key"`
	ReadHeaderTimeoutSeconds int    `json:"read_header_timeout_seconds"`
	IdleTimeoutSeconds       int    `json:"idle_timeout_seconds"`
	ReadTimeoutSeconds       int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds      int    `json:"write_timeout_seconds"`
	ShutdownTimeoutSeconds   int    `json:"shutdown_timeout_seconds"`
}
