package output

import (
	"encoding/json"
	"io"
)

// JSONErrorCode identifies the category of a structured JSON error.
type JSONErrorCode string

const (
	ErrCodeAuth       JSONErrorCode = "auth_expired"
	ErrCodePermission JSONErrorCode = "permission_denied"
	ErrCodeNotFound   JSONErrorCode = "not_found"
	ErrCodeNetwork    JSONErrorCode = "network_error"
	ErrCodeReadOnly   JSONErrorCode = "read_only"
	ErrCodeValidation JSONErrorCode = "validation_error"
	ErrCodeInternal   JSONErrorCode = "internal_error"
)

// JSONError is the structured error envelope emitted when --json is active.
type JSONError struct {
	Error JSONErrorDetail `json:"error"`
}

// JSONErrorDetail holds the fields inside the "error" key.
type JSONErrorDetail struct {
	Code       JSONErrorCode `json:"code"`
	Message    string        `json:"message"`
	Suggestion string        `json:"suggestion,omitempty"`
}

// PrintJSONError writes a structured JSON error to w.
func PrintJSONError(w io.Writer, code JSONErrorCode, message, suggestion string) {
	je := JSONError{
		Error: JSONErrorDetail{
			Code:       code,
			Message:    message,
			Suggestion: suggestion,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(je)
}
