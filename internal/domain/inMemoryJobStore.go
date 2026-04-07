package domain

import (
	"sync"
	"time"
)

type InMemJobStore struct {
	mu    *sync.RWMutex
	store map[string]JobRecord
}

func NewInMemJobStore() *InMemJobStore {
	return &InMemJobStore{
		mu:    new(sync.RWMutex),
		store: make(map[string]JobRecord),
	}
}

var jobStore JobStore = NewInMemJobStore()

func GetJobStoreInstance() *JobStore {
	return &jobStore
}


func (js InMemJobStore) GetRecord(id string) JobRecord {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.store[id]
}

func (js InMemJobStore) AddNewRecord(job Job) {
	js.mu.Lock()

	jobRecord := JobRecord{
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