package shared

import "context"

type Broker interface {
	Submit(ctx context.Context, job *Job) error
	Cancel(ctx context.Context, jobId string) error
	Acquire(ctx context.Context) (*Job, string, error)
	MarksAsCompleted(ctx context.Context, jobID, leaseID string) error
	IsCancelled(ctx context.Context, jobID string) (bool, error)
	RemoveCancelled(ctx context.Context, jobID string) error

	// PollReadyJobs transfers ready to execute jobs from main_queue to ready_queue
	PollReadyJobs(ctx context.Context, batchSize int) error

	// RecoverAbandonedJobs attempts to recover jobs that have been abandoned or stuck in processing by re-queuing them.
	RecoverAbandonedJobs(ctx context.Context, batchSize int) error
}
