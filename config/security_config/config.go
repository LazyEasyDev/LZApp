package security_config

type SecurityConfig struct {
	HMACKey        string `json:"-"`
	HMACTokenBytes int    `json:"hmac_token_bytes"` // 16|24|32
}
