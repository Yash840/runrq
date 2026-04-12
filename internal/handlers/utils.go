package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Yash840/runrq/internal/dto"
)

func HandleError(w http.ResponseWriter, status int, path string, inErr error) {
	msg := fmt.Sprintf("request processing failed : %v", inErr)
	apiResponse := dto.NewFailedApiResponse(path, nil, status, msg)

	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(&apiResponse)
	if err != nil {
		log.Fatal(err)
	}
}