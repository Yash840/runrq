package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Yash840/runrq/internal/domain"
	"github.com/Yash840/runrq/internal/dto"
	"github.com/Yash840/runrq/internal/engine"
	"github.com/Yash840/runrq/internal/model"
	"github.com/Yash840/runrq/internal/repository"
	"github.com/google/uuid"
)

type JobRecordHandler struct {
	repo       *repository.JobRecordsRepo
	dispatcher *engine.Dispatcher
}

func (handler *JobRecordHandler) CreateJobRecord(w http.ResponseWriter, r *http.Request) {
	var jobReq dto.SubmitJobReq
	err := json.NewDecoder(r.Body).Decode(&jobReq)
	if err != nil {
		HandleError(w, http.StatusBadRequest, r.RequestURI, err)
		return
	}

	jobId := uuid.NewString()

	jobRecord := model.JobRecord{ID: jobId, Status: "Pending", CreatedAt: time.Now()}

	err = handler.repo.Create(jobRecord)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, r.RequestURI, err)
		return
	}

	job := domain.Job{ID: jobId, Payload: []byte(jobReq.Payload), Type: jobReq.Type}
	handler.dispatcher.Submit(job)

	responseData := struct {
		JobId string `json:"jobId"`
	}{JobId: jobId}

	apiResponse := dto.NewSuccessApiResponse(r.RequestURI, responseData, http.StatusAccepted, "job created successfully")

	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(&apiResponse)
	if err != nil {
		log.Printf("failed to encode and write response: %v", err)
		return
	}
}

func (handler *JobRecordHandler) GetJobRecord(w http.ResponseWriter, r *http.Request) {
	jobId := r.PathValue("id")

	jobRecord, err := handler.repo.Get(jobId)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, r.RequestURI, err)
		return
	}

	apiResponse := dto.NewSuccessApiResponse(r.RequestURI, jobRecord, http.StatusOK, "")

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(&apiResponse)
	if err != nil {
		log.Printf("failed to encode and write response: %v", err)
		return
	}
}
