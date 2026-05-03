package shared

import "context"

type Broker interface {
	Submit(ctx context.Context, job *Job) error
	Cancel(ctx context.Context, jobId string) error
	Acquire(ctx context.Context) (*Job, error)
	Ack(ctx context.Context, jobID string) error
	IsCancelled(ctx context.Context, jobID string) (bool, error)

	// PollReadyJobs transfers ready to execute jobs from main_queue to ready_queue
	PollReadyJobs(ctx context.Context, batchSize int) error
}
