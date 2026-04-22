package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Yash840/runrq/internal/domain"
	"github.com/Yash840/runrq/internal/engine"
	"github.com/Yash840/runrq/internal/repository"
	"github.com/google/uuid"
)

func HandleJobSubmission(d *engine.Dispatcher, js *repository.JobRecordsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.SubmitJobReq
		err := json.NewDecoder(r.Body).Decode(&req)

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		jobId := uuid.NewString()

		job := domain.Job{
			ID:      jobId,
			Type:    req.Type,
			Payload: []byte(req.Payload),
		}

		d.Submit(job)

		(*js).AddNewRecord(job)

		w.WriteHeader(http.StatusAccepted)

		json.NewEncoder(w).Encode(fmt.Sprintf("{'status': 'Accepted', 'ID': '%s'}", jobId))
	}
}
