package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Yash840/runrq/internal"
	"github.com/Yash840/runrq/internal/http/dto"
	"github.com/Yash840/runrq/internal/shared"
)

const APIBaseURL = "/api/v1/jobs"

func RegisterJobHandlers(mux *http.ServeMux, c *internal.Client) {
	mux.HandleFunc("GET /", Health())
	mux.HandleFunc(fmt.Sprintf("POST %s", APIBaseURL), HandleJobSubmit(c))
	mux.HandleFunc(fmt.Sprintf("DELETE %s/{jobID}", APIBaseURL), HandleJobCancel(c))
}

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)

		err := json.NewEncoder(w).Encode(new(dto.NewSuccessApiResponse(http.StatusOK, r.RequestURI, "Service Running", nil)))
		if err != nil {
			log.Printf("failed to encode and write response: %v", err)
			return
		}
	}
}

func HandleJobSubmit(c *internal.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var opts shared.JobOpts
		err := json.NewDecoder(r.Body).Decode(&opts)
		if err != nil {
			err = json.NewEncoder(w).Encode(new(dto.NewFailedApiResponse(http.StatusAccepted, r.RequestURI, "Internal Error")))
			if err != nil {
				log.Printf("failed to encode and write response: %v", err)
				return
			}
			return
		}

		jobID, err := c.Submit(opts)
		if err != nil {
			err = json.NewEncoder(w).Encode(new(dto.NewFailedApiResponse(http.StatusInternalServerError, r.RequestURI, err.Error())))
			if err != nil {
				log.Printf("failed to encode and write response: %v", err)
				return
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)

		err = json.NewEncoder(w).Encode(new(dto.NewSuccessApiResponse(http.StatusAccepted, r.RequestURI, "Job Submitted", jobID)))
		if err != nil {
			log.Printf("failed to encode and write response: %v", err)
			return
		}
	}
}

func HandleJobCancel(c *internal.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("jobID")

		err := c.Cancel(jobID)
		if err != nil {
			err = json.NewEncoder(w).Encode(new(dto.NewFailedApiResponse(http.StatusInternalServerError, r.RequestURI, err.Error())))
			if err != nil {
				log.Printf("failed to encode and write response: %v", err)
				return
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(new(dto.NewSuccessApiResponse(http.StatusAccepted, r.RequestURI, "Job Cancellation Requested", nil)))
		if err != nil {
			log.Printf("failed to encode and write response: %v", err)
			return
		}
	}
}
