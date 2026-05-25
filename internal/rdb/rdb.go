package rdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Yash840/runrq/internal/shared"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

/*
Queue Structure:

In Runrq v1, there are four primary queues for storing and managing jobs:

1. main_queue: The entry point of the job lifecycle. Every newly submitted job goes here.
   In this implementation, this queue is a Redis Sorted Set (ZSET).
   Jobs are stored in this queue as:
     - key: 'main_queue'
     - score: nextRunAt (execution timestamp)
     - member: jobKey ("job:" + jobID)

2. ready_queue: Jobs from the main_queue whose nextRunAt (execution time) has arrived are transferred to this queue.
   This queue is a Redis LIST. Only the jobKey is stored in this queue.

3. processing_queue: When a worker acquires a job from the ready_queue, it is transferred to this queue to track it for recovery and retry purposes.
   This queue is also a Redis ZSET, storing jobs as:
     - key: 'processing_queue'
     - score: deadline (computed during job acquisition and can be extended by the worker via heartbeating)
     - member: jobKey

4. failed_queue: When a job fails (e.g. exhausts all retries), it is moved to this queue for tracking.
   This queue is a Redis ZSET, storing jobs as:
     - key: 'failed_queue'
     - score: failure timestamp
     - member: jobKey

While these queues primarily manage the jobKeys, the actual job data is stored as a Redis HASH using the jobKey as the identifier.
Cancellation is handled by updating the 'status' field of this hash to 'Cancelled' instead of maintaining a separate Set.

It is stored as:
  - hash key: jobKey
  - field 'status': current status of the job
  - field 'lease_id': unique ID for job ownership during processing
  - field '...': other job details as separate fields
*/

const (
	MainQueue       = "main_queue"
	ReadyQueue      = "ready_queue"
	ProcessingQueue = "processing_queue"
	FailedQueue     = "failed_queue"

	DefaultLeaseDuration = 30 * time.Second
	MaxRecoveryBackoff   = 30 * time.Second
)

// RDB represents the Redis-backed broker implementation for managing job queues.
// It wraps a Redis client to interact with the underlying database.
type RDB struct {
	client *redis.Client
}

func NewRDB(client *redis.Client) *RDB {
	return &RDB{client: client}
}

// submitScript atomically adds a job to the main_queue and sets its hash data.
// KEYS[1]: MainQueue, KEYS[2]: jobKey
// ARGV: score, job fields (key, value, key, value, ...)
var submitScript = redis.NewScript(`
	redis.call('ZADD', KEYS[1], ARGV[1], KEYS[2])
	redis.call('HSET', KEYS[2], unpack(ARGV, 2))
	return 1
`)

// Submit adds a new job to the main_queue and sets its initial data.
func (rdb *RDB) Submit(ctx context.Context, job *shared.Job) error {
	nextRunAt := job.NextRunAt.Unix()
	jobKey := generateJobKey(job.JobID)

	payloadJSON, err := json.Marshal(job.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	err = submitScript.Run(ctx, rdb.client, []string{MainQueue, jobKey}, nextRunAt,
		"job_id", job.JobID,
		"owner_id", job.OwnerID,
		"job_type", job.JobType,
		"payload", string(payloadJSON),
		"status", job.Status.String(),
		"schedule_time", job.ScheduleTime.Format(time.RFC3339),
		"next_run_at", job.NextRunAt.Format(time.RFC3339),
		"retry_policy", job.RetryPolicy.String(),
		"max_retries", job.MaxRetries,
		"retries_done", job.RetriesDone,
		"store_result", job.StoreResult,
		"max_timeout", job.MaxTimeout,
	).Err()
	if err != nil {
		return fmt.Errorf("failed to submit job : %w", err)
	}

	return nil
}

// cancelScript atomically tries to remove a job from queues or marks it as cancelled.
// KEYS[1]: MainQueue, KEYS[2]: ReadyQueue, KEYS[3]: jobKey
var cancelScript = redis.NewScript(`
	local removed = redis.call('ZREM', KEYS[1], KEYS[3])
	if removed > 0 then
		redis.call('DEL', KEYS[3])
		return {}
	end
	
	removed = redis.call('LREM', KEYS[2], 0, KEYS[3])
	if removed > 0 then
		redis.call('DEL', KEYS[3])
		return {}
	end

	redis.call('HSET', KEYS[3], 'status', 'Cancelled') 
	return {}
`)

func (rdb *RDB) Cancel(ctx context.Context, jobID string) error {
	jobKey := generateJobKey(jobID)
	err := cancelScript.Run(ctx, rdb.client, []string{MainQueue, ReadyQueue, jobKey}).Err()
	if err != nil {
		return fmt.Errorf("failed to cancel job : %w", err)
	}
	return nil
}

func (rdb *RDB) IsCancelled(ctx context.Context, jobID string) (bool, error) {
	jobKey := generateJobKey(jobID)
	jobStatus, err := rdb.client.HGet(ctx, jobKey, "status").Result()
	if err != nil {
		return false, fmt.Errorf("failed to fetch job : %w", err)
	}
	return jobStatus == "Cancelled", nil
}

// acquisitionScript atomically acquires a job, moves it to processing, sets status and lease.
// KEYS[1]: ReadyQueue, KEYS[2]: ProcessingQueue
// ARGV[1]: currentTime, ARGV[2]: timeout (seconds), ARGV[3]: leaseID
var acquisitionScript = redis.NewScript(`
	local jobKey = redis.call('RPOP', KEYS[1])
	if jobKey == nil then
		return nil
	end

	local deadline = ARGV[1] + ARGV[2]
	redis.call('ZADD', KEYS[2], deadline, jobKey)
	redis.call('HSET', jobKey, 'status', 'Processing', 'lease_id', ARGV[3])
	return jobKey
`)

func (rdb *RDB) Acquire(ctx context.Context) (*shared.Job, string, error) {
	leaseID, err := generateLeaseID()
	if err != nil {
		return nil, "", err
	}

	now := time.Now().Unix()
	res, err := acquisitionScript.Run(ctx, rdb.client, []string{ReadyQueue, ProcessingQueue}, now, DefaultLeaseDuration.Seconds(), leaseID).Result()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch job : %w", err)
	}

	if res == nil {
		return nil, "", ErrNoJobsToAcquire{Code: 500, Message: "no jobs to acquire"}
	}

	jobKey := res.(string)

	type JobScan struct {
		shared.Job
		Payload string `redis:"payload"`
	}
	var js JobScan
	err = rdb.client.HGetAll(ctx, jobKey).Scan(&js)
	if err != nil {
		return nil, "", fmt.Errorf("failed to scan job: %w", err)
	}

	job := js.Job
	if js.Payload != "" {
		var p any
		if err := json.Unmarshal([]byte(js.Payload), &p); err == nil {
			job.Payload = p
		}
	}

	return &job, leaseID, nil
}

// completionScript atomically verifies lease, removes job from processing, and cleans up data.
// KEYS[1]: jobKey, KEYS[2]: ProcessingQueue
// ARGV[1]: leaseID
var completionScript = redis.NewScript(`
	local currentLease = redis.call('HGET', KEYS[1], 'lease_id')
	if currentLease ~= ARGV[1] then
		error('lease expired')
	end

	redis.call('ZREM', KEYS[2], KEYS[1])
	redis.call('DEL', KEYS[1])
	return {}
`)

func (rdb *RDB) MarksAsCompleted(ctx context.Context, jobID, leaseID string) error {
	err := completionScript.Run(ctx, rdb.client, []string{generateJobKey(jobID), ProcessingQueue}, leaseID).Err()
	if err != nil {
		return fmt.Errorf("failed to mark job succeed: %w", err)
	}
	return nil
}

var failureScript = redis.NewScript(`
	local currentLease = redis.call('HGET', KEYS[2], 'lease_id')
	if currentLease ~= ARGV[2] then
		error('lease expired')
	end

	-- Add to failed_queue
	redis.call('ZADD', KEYS[1], ARGV[1], KEYS[2])

	-- Remove from processing_queue
	redis.call('ZREM', KEYS[3], KEYS[2])

	-- Update status to Failed
	redis.call('HSET', KEYS[2], 'status', 'Failed')

	-- Clear lease
	redis.call('HDEL', KEYS[2], 'lease_id')

	return {}
`)

func (rdb *RDB) MarkAsFailed(ctx context.Context, jobID, leaseID string) error {
	failedAt := time.Now().Unix()

	err := failureScript.Run(ctx, rdb.client, []string{FailedQueue, generateJobKey(jobID), ProcessingQueue}, failedAt, leaseID).Err()
	if err != nil {
		return fmt.Errorf("failed to mark job failed: %w", err)
	}
	return nil
}

var cancellationScript = redis.NewScript(`
	redis.call('ZREM', KEYS[1], KEYS[2])
	redis.call('DEL', KEYS[2])
	return {}
`)

func (rdb *RDB) RemoveCancelled(ctx context.Context, jobID string) error {
	err := cancellationScript.Run(ctx, rdb.client, []string{ProcessingQueue, generateJobKey(jobID)})
	if err != nil {
		return fmt.Errorf("failed to remove cancelled job: %w", err)
	}
	return nil
}

// leaseExtensionScript atomically extends the deadline of a specific job actively being processed.
// KEYS[1]: jobKey, KEYS[2]: ProcessingQueue
// ARGV[1]: leaseID, ARGV[2]: extension (seconds)
var leaseExtensionScript = redis.NewScript(`
	local leaseID = redis.call('HGET', KEYS[1], 'lease_id')
	if leaseID ~= ARGV[1] then
		error("invalid lease")
	end

	redis.call('ZADD', KEYS[2], ARGV[2], KEYS[1])
	return {}
`)

func (rdb *RDB) ExtendDeadline(ctx context.Context, jobID, leaseID string, extension time.Duration) error {
	now := time.Now().Unix()
	newDeadline := now + int64(extension.Seconds())

	err := leaseExtensionScript.Run(ctx, rdb.client, []string{generateJobKey(jobID), ProcessingQueue}, leaseID, newDeadline).Err()
	if err != nil {
		return fmt.Errorf("failed to extend deadline: %w", err)
	}
	return nil
}

// resubmissionScript atomically moves a job from processing back to main_queue and updates its status.
// KEYS[1]: ProcessingQueue, KEYS[2]: MainQueue, KEYS[3]: jobKey
// ARGV[1]: nextRunAt (score), ARGV[2...]: fields
var resubmissionScript = redis.NewScript(`
	redis.call('ZREM', KEYS[1], KEYS[3])
	redis.call('ZADD', KEYS[2], ARGV[1], KEYS[3])
	redis.call('HDEL', KEYS[3], 'lease_id')
	redis.call('HSET', KEYS[3], unpack(ARGV, 2))
	return {}
`)

func (rdb *RDB) ResubmitJob(ctx context.Context, nextRunAfter time.Duration, job *shared.Job) error {
	now := time.Now().Unix()
	nextRunAt := now + int64(nextRunAfter.Seconds())
	jobKey := generateJobKey(job.JobID)

	err := resubmissionScript.Run(ctx, rdb.client, []string{ProcessingQueue, MainQueue, jobKey}, nextRunAt,
		"status", "Retrying",
		"retries_done", job.RetriesDone,
		"next_run_at", job.NextRunAt.Format(time.RFC3339),
	).Err()
	if err != nil {
		return fmt.Errorf("failed to resubmit job: %w", err)
	}
	return nil
}

// transferScript atomically moves ready jobs from main_queue to ready_queue.
var transferScript = redis.NewScript(`
	local jobs = redis.call('ZRANGE', KEYS[1], '-inf', ARGV[1], 'BYSCORE', 'LIMIT', 0, ARGV[2])
	if #jobs > 0 then 
		redis.call('ZREM', KEYS[1], unpack(jobs))
		redis.call('LPUSH', KEYS[2], unpack(jobs))
	end
	return #jobs
`)

func (rdb *RDB) PollReadyJobs(ctx context.Context, batchSize int) error {
	now := time.Now().Unix()
	res, err := transferScript.Run(ctx, rdb.client, []string{MainQueue, ReadyQueue}, now, batchSize).Result()
	if err != nil && err.Error() != "redis: nil" {
		return fmt.Errorf("failed to poll jobs : %w", err)
	}

	cnt := res.(int64)
	if cnt == 0 {
		return ErrNoJobsToPoll{Code: 500, Message: "no jobs ready for execution"}
	}

	return nil
}

// recoverScript atomically requeues abandoned jobs back into main_queue with random backoff.
var recoverScript = redis.NewScript(`
	local jobs = redis.call('ZRANGE', KEYS[1], '-inf', ARGV[1], 'BYSCORE', 'LIMIT', 0, ARGV[2])
	if #jobs > 0 then
		for i, jobKey in ipairs(jobs) do
			local jobStatus = redis.call('HGET', jobKey, 'status')
			local newScore = ARGV[1] + math.random(ARGV[3])
			redis.call('ZREM', KEYS[1], jobKey)
			redis.call('HDEL', jobKey, 'lease_id')

			if jobStatus ~= 'Cancelled' then
				redis.call('ZADD', KEYS[2], newScore, jobKey)
				redis.call('HSET', jobKey, 'status', "Recovering")
			else 
				redis.call('DEL', jobKey)
			end		
		end
	end
	return {}
`)

func (rdb *RDB) RecoverAbandonedJobs(ctx context.Context, batchSize int) error {
	now := time.Now().Unix()
	err := recoverScript.Run(ctx, rdb.client, []string{ProcessingQueue, MainQueue}, now, batchSize, MaxRecoveryBackoff.Seconds()).Err()
	if err != nil {
		return fmt.Errorf("failed to recover jobs : %w", err)
	}
	return nil
}

func generateJobKey(ID string) string {
	return "job:" + ID
}

func generateLeaseID() (string, error) {
	b, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

type ErrNoJobsToAcquire struct {
	Code    int
	Message string
}

func (e ErrNoJobsToAcquire) Error() string {
	return fmt.Sprintf("error %d: %s", e.Code, e.Message)
}

type ErrNoJobsToPoll struct {
	Code    int
	Message string
}

func (e ErrNoJobsToPoll) Error() string {
	return fmt.Sprintf("error %d: %s", e.Code, e.Message)
}
