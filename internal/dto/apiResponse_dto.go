package dto

import "time"

type ApiResponse struct {
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	Message   string    `json:"message"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

func NewSuccessApiResponse(path string, data any, status int, message string) ApiResponse {
	mes := "request processed successfully"
	if message != "" {
		mes = message
	}

	return ApiResponse{
		Path:      path,
		Status:    status,
		Message:   mes,
		Success:   true,
		Timestamp: time.Now(),
		Data:      data,
	}
}

func NewFailedApiResponse(path string, data any, status int, message string) ApiResponse {
	mes := "request processing failed"
	if message != "" {
		mes = message
	}

	return ApiResponse{
		Path:      path,
		Status:    status,
		Message:   mes,
		Success:   false,
		Timestamp: time.Now(),
		Data:      nil,
	}
}
