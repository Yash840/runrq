package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Yash840/runrq/internal/domain"
)

func HandleJobRetrieval(js *domain.JobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobId := r.PathValue("id")

		if jobId == "" {
			http.Error(w, "must pass job id to fetch details", http.StatusBadRequest)
			return
		}

		jobRecord := (*js).GetRecord(jobId)

		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(jobRecord)
	}
}
