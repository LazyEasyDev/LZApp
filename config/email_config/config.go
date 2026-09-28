package email_config

type EmailConfig struct {
	FromEmail      string `json:"from_email"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
