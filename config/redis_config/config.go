package redis_config

type RedisConfig struct {
	ClusterMode         bool     `json:"cluster_mode"`
	Addrs               []string `json:"addrs"`
	Username            string   `json:"username"`
	Password            string   `json:"-"`
	PasswordFile        string   `json:"-"`
	KeyPrefix           string   `json:"key_prefix"`
	TLS                 bool     `json:"tls"`
	InsecureSkipVerify  bool     `json:"insecure_skip_verify"`
	CACertFile          string   `json:"ca_cert_file"`
	ServerName          string   `json:"server_name"`
	DialTimeoutSeconds  int      `json:"dial_timeout_seconds"`
	ReadTimeoutSeconds  int      `json:"read_timeout_seconds"`
	WriteTimeoutSeconds int      `json:"write_timeout_seconds"`
}
