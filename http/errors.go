package http

import (
	"encoding/json"
	"fmt"
)

// ParseError attempts to parse a JSON error response.
func ParseError(responseBody []byte) (*ErrorResponse, error) {
	var errResp ErrorResponse
	if err := json.Unmarshal(responseBody, &errResp); err != nil {
		return nil, fmt.Errorf("failed to parse error response: %w", err)
	}
	return &errResp, nil
}
