package security_config

type SecurityHMACConfig struct {
	HMACKey        string `json:"-"`
	HMACTokenBytes int    `json:"hmac_token_bytes"` // 16|24|32

}

type SecurityBcryptConfig struct {
	Cost int `json:"cost"`
}
