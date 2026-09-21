package httpapi

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}
