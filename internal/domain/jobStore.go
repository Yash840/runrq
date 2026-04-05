package domain

type JobStore interface {
	GetRecord(id string) JobRecord
	AddNewRecord(job Job)
	MakeJobCompleted(id string, result any)
	MakeJobProcessing(id string)
	MakeJobFailed(id string, err string)
}