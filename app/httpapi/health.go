package httpapi

type healthOutput struct {
	Body struct {
		ServerUnixTime int64 `json:"server_unix_time" example:"1680000000"`
	}
}
