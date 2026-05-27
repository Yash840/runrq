package dto

import "time"

type ApiResponse struct {
	Success   bool      `json:"success"`
	Status    int       `json:"status"`
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Data      any       `json:"data"`
}

func NewApiResponse(success bool, status int, path string, message string, data any) ApiResponse {
	if message == "" {
		if success {
			message = "Request Successful"
		} else {
			message = "Request Failed"
		}
	}

	return ApiResponse{
		Success:   success,
		Status:    status,
		Path:      path,
		Timestamp: time.Now(),
		Message:   message,
		Data:      data,
	}
}

func NewSuccessApiResponse(status int, path string, message string, data any) ApiResponse {
	return NewApiResponse(true, status, path, message, data)
}

func NewFailedApiResponse(status int, path string, message string) ApiResponse {
	return NewApiResponse(false, status, path, message, nil)
}
