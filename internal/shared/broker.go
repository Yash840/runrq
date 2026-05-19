package shared

import (
	"context"
	"time"
)

// Broker represents the interface for a job brokering system.
// It manages the lifecycle of jobs, including their submission, cancellation, acquisition, and completion.
type Broker interface {
	// Submit adds a new job to the broker for execution.
	Submit(ctx context.Context, job *Job) error

	// Cancel marks a specific job as cancelled using its job ID.
	Cancel(ctx context.Context, jobId string) error

	// Acquire fetches a job ready for execution and returns the job along with a unique lease ID.
	Acquire(ctx context.Context) (*Job, string, error)

	// MarksAsCompleted flags a job as successfully finished, using the job ID and its corresponding lease ID.
	MarksAsCompleted(ctx context.Context, jobID, leaseID string) error

	// IsCancelled checks whether a particular job has been marked as cancelled.
	IsCancelled(ctx context.Context, jobID string) (bool, error)

	// RemoveCancelled permanently deletes a cancelled job from the broker.
	RemoveCancelled(ctx context.Context, jobID string) error

	// ExtendDeadline extends the lease or execution timeframe for a currently acquired job.
	ExtendDeadline(ctx context.Context, jobID, leaseID string, extension time.Duration) error

	// ResubmitJob resubmits a job for execution after a specific duration.
	ResubmitJob(ctx context.Context, jobID string, nextRunAfter time.Duration, jobJSON string) error

	// PollReadyJobs transfers ready to execute jobs from main_queue to ready_queue
	PollReadyJobs(ctx context.Context, batchSize int) error

	// RecoverAbandonedJobs attempts to recover jobs that have been abandoned or stuck in processing by re-queuing them.
	RecoverAbandonedJobs(ctx context.Context, batchSize int) error
}
