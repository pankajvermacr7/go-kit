package http

// ErrorResponse represents a structured error response.
type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}
