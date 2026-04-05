package engine

import (
	"sync"
	"time"
	"github.com/Yash840/runrq/internal/domain"
)

type InMemJobStore struct {
	mu    *sync.RWMutex
	store map[string]domain.JobRecord
}

func NewInMemJobStore() *InMemJobStore {
	return &InMemJobStore{
		mu:    new(sync.RWMutex),
		store: make(map[string]domain.JobRecord),
	}
}

func (js InMemJobStore) GetRecord(id string) domain.JobRecord {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.store[id]
}

func (js InMemJobStore) AddNewRecord(job domain.Job) {
	js.mu.Lock()

	jobRecord := domain.JobRecord{
		ID:        job.ID,
		Status:    "Pending",
		Result:    nil,
		Error:     "",
		CreatedAt: time.Now(),
	}
	js.store[job.ID] = jobRecord

	js.mu.Unlock()
}


func (js InMemJobStore) MakeJobCompleted(id string, result any) {
	js.mu.Lock()
	defer js.mu.Unlock()

	jobRecord := js.store[id]
	jobRecord.Result = result
	jobRecord.Status = "Completed"
	jobRecord.CompletedAt = time.Now()
	js.store[id] = jobRecord
}

func (js InMemJobStore) MakeJobProcessing(id string) {
	js.mu.Lock()
	defer js.mu.Unlock()

	jobRecord := js.store[id]
	jobRecord.Status = "Completed"
	js.store[id] = jobRecord
}

func (js InMemJobStore) MakeJobFailed(id string, err string) {
	js.mu.Lock()
	defer js.mu.Unlock()

	jobRecord := js.store[id]
	jobRecord.Status = "Failed"
	jobRecord.Error = err
	js.store[id] = jobRecord
}