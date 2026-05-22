package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/Yash840/runrq/internal/rdb"
	"github.com/Yash840/runrq/internal/repository"
	"github.com/Yash840/runrq/internal/shared"
)

const (
	ImmediateBaseDelay = 1 //Seconds
	ModerateBaseDelay  = 2
	DelayedBaseDelay   = 3

	ImmediateMaxBackoff = 15
	ModerateMaxBackoff  = 20
	DelayedMaxBackoff   = 25

	maxPollingJitter = 5
	pollingBackoff   = 2
)

type Dispatcher struct {
	log     *log.Logger
	broker  shared.Broker
	repo    repository.ResultRepository
	baseCtx context.Context
	quit    <-chan struct{}

	registry *JobRegistry

	cancellations sync.Map

	resBackup sync.Map
}

func (d *Dispatcher) Start() {

}

func (d *Dispatcher) Shutdown() {

}

func (d *Dispatcher) exec() {
	workerID, err := generateWorkerID()
	if err != nil {
		d.log.Printf("error in generating workerID: %s\n", err)
		return
	}

	d.log.Printf("worker #%s: started\n", workerID)

	var noJobsError rdb.ErrNoJobsToAcquire
	for {
		select {
		case <-d.quit:
			d.log.Printf("worker #%s: stopping\n", workerID)
			return
		default:
		}

		job, lease, err := d.broker.Acquire(d.baseCtx)

		switch {
		case errors.As(err, &noJobsError):
			jitter := rand.Intn(maxPollingJitter)
			time.Sleep(time.Duration(jitter) * time.Second)
		case err != nil:
			d.log.Printf("worker #%s: error in job acquisition: %s\n", workerID, err.Error())
			time.Sleep(time.Duration(pollingBackoff) * time.Second)
		default:
			isCancelled, err := d.broker.IsCancelled(d.baseCtx, job.JobID)
			if err != nil {
				d.log.Printf("worker #%s: error in job cancellation check: %s\n", workerID, err.Error())
			} else if isCancelled {
				d.log.Printf("worker #%s: acquired job #%s already cancelled\n", workerID, job.JobID)
				d.broker.RemoveCancelled(d.baseCtx, job.JobID)
				continue
			}

			deadline := time.Now().Add(time.Duration(job.MaxTimeout) * time.Second)
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()

			d.cancellations.Store(job.JobID, cancel)

			err = d.handleJob(ctx, job, lease)
			if err != nil {
				d.log.Printf("worker #%s: error in handling job #%s: %v\n", workerID, job.JobID, err)
			}
		}
	}
}

func (d *Dispatcher) startScheduling() {
	for {
		select {
		case <-d.quit:
			return
		default:
			err := d.broker.PollReadyJobs(d.baseCtx, 30)

			var noJobsError rdb.ErrNoJobsToPoll
			switch {
			case errors.As(err, &noJobsError):
				jitter := rand.Intn(maxPollingJitter)
				time.Sleep(time.Duration(jitter) * time.Second)
			case err != nil:
				d.log.Printf("error in job polling: %s", err.Error())
				time.Sleep(time.Duration(pollingBackoff) * time.Second)
			}
		}
	}
}

func (d *Dispatcher) startRecovery() {
	for {
		select {
		case <-d.quit:
			return
		default:
			err := d.broker.RecoverAbandonedJobs(d.baseCtx, 10)
			if err != nil {
				d.log.Printf("error in job recovery: %s", err.Error())
				time.Sleep(time.Duration(pollingBackoff) * time.Second)
			}
		}
	}
}

func (d *Dispatcher) handleJob(ctx context.Context, job *shared.Job, lease string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		processor, err := d.registry.GetProc(job.JobType)
		if err != nil {
			return err
		}

		var res any
		res, err = processor.Process(job)

		if err != nil {
			// Retry Pipeline
			d.log.Printf("job #%s execution failed\n", job.JobID)

			// Handling not eligible jobs to retry
			if job.RetryPolicy == shared.RetryPolicyNone || job.RetriesDone == job.MaxRetries {
				d.log.Printf("job #%s not eligible for retry it is marked as failed\n", job.JobID)

				err = d.broker.MarkAsFailed(ctx, job.JobID, lease)
				if err != nil {
					d.log.Printf("job #%s failed to mark as failed\n", job.JobID)
					return err
				}
			}

			// Handling eligible jobs to retry
			nextRunAfter := calculateBackoff(job.RetryPolicy, job.RetriesDone)
			err = d.broker.ResubmitJob(ctx, nextRunAfter, job)
			if err != nil {
				d.log.Printf("job #%s failed to resubmit for retry\n", job.JobID)
				return err
			}
		}

		// Completion Pipeline
		err = d.broker.MarksAsCompleted(ctx, job.JobID, lease)
		if err != nil {
			d.log.Printf("job #%s execution completed but completion failed: %v", job.JobID, err)
			return err
		}

		if job.StoreResult {
			err = d.repo.Create(job.JobID, res, time.Now())
			if err != nil {
				d.log.Printf("job #%s result failed to store keeping it in backup: %v", job.JobID, err)
				result := shared.Result{
					JobID:      job.JobID,
					FinishTime: time.Now(),
					Body:       res,
				}

				d.resBackup.Store(time.Now().Unix(), result)
			}
		}

		d.log.Printf("job #%s completed\n", job.JobID)

		return nil
	}
}

func generateWorkerID() (string, error) {
	hostName, err := os.Hostname()
	if err != nil {
		return "", nil
	}
	pid := os.Getpid()
	instant := time.Now().UnixMilli()

	return fmt.Sprintf("%s:%d:%d", hostName, pid, instant), nil
}

func calculateBackoff(policy shared.RetryPolicy, retry int) time.Duration {
	var baseDelay int
	var maxBackoff int

	switch policy {
	case shared.RetryPolicyImmediate:
		baseDelay = ImmediateBaseDelay
		maxBackoff = ImmediateMaxBackoff
	case shared.RetryPolicyModerate:
		baseDelay = ModerateBaseDelay
		maxBackoff = ModerateMaxBackoff
	default:
		baseDelay = DelayedBaseDelay
		maxBackoff = DelayedMaxBackoff
	}

	backoff := baseDelay + (1 << retry)

	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	jitter := rand.Int63n(int64(backoff / 2))

	return time.Duration(backoff+jitter) * time.Second
}
