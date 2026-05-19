package rdb

import (
	"context"
	"encoding/json"
	"errors"
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

While these queues primarily manage the jobKeys, the actual job data is stored as a Redis HASH using the jobKey as the identifier.
Cancellation is handled by updating the 'status' field of this hash to 'Cancelled' instead of maintaining a separate Set.

It is stored as:
  - hash key: jobKey
  - field 'status': current status of the job
  - field 'job-json': JSON blob containing the job body
*/

const (
	MainQueue       = "main_queue"
	ReadyQueue      = "ready_queue"
	ProcessingQueue = "processing_queue"

	DefaultJobTimeout  = 30 * time.Second
	MaxRecoveryBackoff = 30 * time.Second
)

var ErrNoJobsToAcquire = errors.New("no jobs to acquire")

// RDB represents the Redis-backed broker implementation for managing job queues.
// It wraps a Redis client to interact with the underlying database.
type RDB struct {
	client *redis.Client
}

// submitScript atomically adds a job to the main_queue and sets its hash data.
// KEYS[1]: MainQueue, KEYS[2]: jobKey
// ARGV[1]: nextRunAt (score), ARGV[2]: jobJSON
var submitScript = redis.NewScript(`
	redis.call('ZADD', KEYS[1], ARGV[1], KEYS[2])
	redis.call('HSET', KEYS[2], 'status', 'Queued', 'job-json', ARGV[2])
	return 1
`)

// Submit adds a new job to the main_queue and sets its initial data.
// It uses a Lua script to atomically:
// 1. Add the job key to the main_queue ZSET with its execution time (nextRunAt) as the score.
// 2. Set the job's initial status to 'Queued' and store its JSON payload in a HASH.
func (rdb *RDB) Submit(ctx context.Context, job *shared.Job) error {
	nextRunAt := job.NextRunAt.Unix()

	jobJSON, err := json.Marshal(job)
	jobKey := generateJobKey(job.JobID)
	if err != nil {
		return fmt.Errorf("failed to submit job : %w", err)
	}

	err = submitScript.Run(ctx, rdb.client, []string{MainQueue, jobKey}, nextRunAt, jobJSON).Err()
	if err != nil {
		return fmt.Errorf("failed to submit job : %w", err)
	}

	return nil
}

// cancelScript atomically tries to remove a job from queues or marks it as cancelled.
// KEYS[1]: MainQueue, KEYS[2]: ReadyQueue, KEYS[3]: jobKey
// ARGV: (none)
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

// Cancel attempts to remove a job from the queues or marks it as cancelled.
// It uses a Lua script to atomically:
// 1. Try removing the job from the main_queue. If successful, delete the job data.
// 2. Try removing the job from the ready_queue. If successful, delete the job data.
// 3. If the job is in neither queue (likely processing), update its status in the job hash to 'Cancelled' for workers to double-check.
func (rdb *RDB) Cancel(ctx context.Context, jobID string) error {
	jobKey := generateJobKey(jobID)
	err := cancelScript.Run(ctx, rdb.client, []string{MainQueue, ReadyQueue, jobKey}).Err()
	if err != nil {
		return fmt.Errorf("failed to cancel job : %w", err)
	}

	return nil
}

// IsCancelled checks if a given jobID has been marked as cancelled.
// It fetches the job hash and checks if the 'status' field is 'Cancelled'.
func (rdb *RDB) IsCancelled(ctx context.Context, jobID string) (bool, error) {
	jobKey := generateJobKey(jobID)
	jobStatus, err := rdb.client.HGet(ctx, jobKey, "status").Result()
	if err != nil {
		return false, fmt.Errorf("failed to fetch job : %w", err)
	}

	isCancelled := jobStatus == "Cancelled"
	return isCancelled, nil
}

// acquisitionScript atomically acquires a job, moves it to processing, sets status and lease.
// KEYS[1]: ReadyQueue, KEYS[2]: ProcessingQueue
// ARGV[1]: currentTime, ARGV[2]: timeout (seconds), ARGV[3]: leaseID
var acquisitionScript = redis.NewScript(`
	local jobKey = redis.call('RPOP', KEYS[1])
	if jobKey == nil then
		return ''
	end

	local deadline = ARGV[1] + ARGV[2]
	redis.call('ZADD', KEYS[2], deadline, jobKey)
	redis.call('HSET', jobKey, 'status', 'Processing')
	local job = redis.call('HGET', jobKey, 'job-json')
	local leaseKey = 'lease:'..jobKey
	redis.call('HSET', leaseKey, 'lease-id', ARGV[3])
	return job
`)

// Acquire fetches a job from the ready_queue for processing.
// It uses a Lua script to atomically:
// 1. Pop a job from the right side of the ready_queue.
// 2. Add the job to the processing_queue with a deadline score (current time + timeout).
// 3. Update the job's status to 'Processing'.
// 4. Generate and store a lease ID for the job to track ownership.
// 5. Return the job's JSON payload.
func (rdb *RDB) Acquire(ctx context.Context) (*shared.Job, string, error) {
	leaseID, err := generateLeaseID()
	if err != nil {
		return nil, "", err
	}

	now := time.Now().Unix()
	res, err := acquisitionScript.Run(ctx, rdb.client, []string{ReadyQueue, ProcessingQueue}, now, DefaultJobTimeout.Seconds(), leaseID).Result()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch job : %w", err)
	}

	if res == "" {
		return nil, "", ErrNoJobsToAcquire
	}

	jobJSON, ok := res.(string)

	if !ok || jobJSON == "" {
		return nil, "", errors.New("no jobs to acquire")
	}

	var job shared.Job
	err = json.Unmarshal([]byte(jobJSON), &job)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal job json: %w", err)
	}

	return &job, leaseID, nil
}

// completionScript atomically verifies lease, removes job from processing, and cleans up data.
// KEYS[1]: leaseKey, KEYS[2]: ProcessingQueue
// ARGV[1]: leaseID, ARGV[2]: jobKey
var completionScript = redis.NewScript(`
	local currentLease = redis.call('HGET', KEYS[1], 'lease-id')
	if currentLease ~= ARGV[1] then
		error('lease expired')
	end

	redis.call('ZREM', KEYS[2], ARGV[2])
	redis.call('DEL', ARGV[2])
	redis.call('DEL', KEYS[1])
	return 1
`)

// MarksAsCompleted removes a finished job from the processing_queue and deletes its data.
// It uses a Lua script to atomically:
// 1. Verify the provided lease ID matches the current lease stored in Redis.
// 2. If valid, remove the job from the processing_queue.
// 3. Delete the job data and its lease.
func (rdb *RDB) MarksAsCompleted(ctx context.Context, jobID, leaseID string) error {
	err := completionScript.Run(ctx, rdb.client, []string{generateLeaseKey(jobID), ProcessingQueue}, leaseID, generateJobKey(jobID)).Err()
	if err != nil {
		return fmt.Errorf("failed to mark job succeed: %w", err)
	}

	return nil
}

// RemoveCancelled completely removes a cancelled job's data from Redis.
// It deletes the job's hash data using its unique job key.
func (rdb *RDB) RemoveCancelled(ctx context.Context, jobID string) error {
	err := rdb.client.Del(ctx, generateJobKey(jobID)).Err()
	if err != nil {
		return fmt.Errorf("failed to remove cancelled job: %w", err)
	}

	return nil
}

// leaseExtensionScript atomically extends the deadline of a specific job actively being processed.
// KEYS[1]: leaseKey, KEYS[2]: ProcessingQueue
// ARGV[1]: leaseID, ARGV[2]: extension (seconds), ARGV[3]: jobKey
var leaseExtensionScript = redis.NewScript(`
	local leaseID = redis.call('HGET', KEYS[1], 'lease-id')
	if leaseID ~= ARGV[1] then
		error("invalid lease")
	end

	redis.call('ZINCRBY', KEYS[2], ARGV[2], ARGV[3])
	return {}
`)

// ExtendDeadline increases the deadline of a job in the processing_queue to prevent it from being recovered.
// It uses a Lua script to atomically:
// 1. Verify the provided lease ID matches the current lease stored in Redis.
// 2. If valid, increment the job's score in the processing_queue by the extension duration.
func (rdb *RDB) ExtendDeadline(ctx context.Context, jobID, leaseID string, extension time.Duration) error {
	err := leaseExtensionScript.Run(ctx, rdb.client, []string{generateLeaseKey(jobID), ProcessingQueue}, leaseID, extension.Seconds(), generateJobKey(jobID)).Err()
	if err != nil {
		return fmt.Errorf("failed to extend deadline: %w", err)
	}

	return nil
}

// resubmissionScript atomically moves a job from processing back to main_queue and updates its status.
// KEYS[1]: ProcessingQueue, KEYS[2]: MainQueue, KEYS[3]: jobKey, KEYS[4]: leaseKey
// ARGV[1]: nextRunAt (score)
var resubmissionScript = redis.NewScript(`
	-- removing from Processing Queue
	redis.call('ZREM', KEYS[1], KEYS[3])

	-- adding back to Main Queue
	redis.call('ZADD', KEYS[2], ARGV[1], KEYS[3])

	-- clearing lease
	redis.call('DEL', KEYS[4])

	-- updating status
	redis.call('HSET', KEYS[3], 'status', 'Retrying', 'job-json', ARGV[2])
	
	return {}
`)

// ResubmitJob returns a processing job back to the main_queue to be retried later.
// It uses a Lua script to atomically:
// 1. Remove the job from the processing_queue.
// 2. Add the job back to the main_queue with a new execution time (nextRunAt).
// 3. Delete the job's lease.
// 4. Update the job's status to 'Retrying'.
func (rdb *RDB) ResubmitJob(ctx context.Context, jobID string, nextRunAfter time.Duration, jobJSON string) error {
	now := time.Now().Unix()
	nextRunAt := now + int64(nextRunAfter.Seconds())

	err := resubmissionScript.Run(ctx, rdb.client, []string{ProcessingQueue, MainQueue, generateJobKey(jobID), generateLeaseKey(jobID)}, nextRunAt, jobJSON).Err()
	if err != nil {
		return fmt.Errorf("failed to resubmit job: %w", err)
	}

	return nil
}

// transferScript atomically moves ready jobs from main_queue to ready_queue.
// KEYS[1]: MainQueue, KEYS[2]: ReadyQueue
// ARGV[1]: currentTime, ARGV[2]: batchSize
var transferScript = redis.NewScript(`
	local jobs = redis.call('ZRANGE', KEYS[1], '-inf', ARGV[1], 'BYSCORE', 'LIMIT', 0, ARGV[2])
	if #jobs == 0 then
		return error("no jobs to poll")
	end

	if #jobs > 0 then 
		redis.call('ZREM', KEYS[1], unpack(jobs))
		redis.call('LPUSH', KEYS[2], unpack(jobs))

		return jobs
	end

	return {}
	`)

// PollReadyJobs moves jobs that are ready to execute from the main_queue to the ready_queue.
// It uses a Lua script to atomically:
// 1. Fetch up to 'batchSize' jobs from the main_queue whose scores (nextRunAt) are less than or equal to the current time.
// 2. Remove these jobs from the main_queue.
// 3. Push these jobs onto the left side of the ready_queue list.
func (rdb *RDB) PollReadyJobs(ctx context.Context, batchSize int) error {
	now := time.Now().Unix()
	err := transferScript.Run(ctx, rdb.client, []string{MainQueue, ReadyQueue}, now, batchSize).Err()
	if err != nil {
		return fmt.Errorf("failed to poll jobs : %w", err)
	}

	return nil
}

// recoverScript atomically requeues abandoned jobs back into main_queue with random backoff.
// KEYS[1]: ProcessingQueue, KEYS[2]: MainQueue
// ARGV[1]: currentTime, ARGV[2]: batchSize, ARGV[3]: maxRecoveryBackoff (seconds)
var recoverScript = redis.NewScript(`
	local jobs = redis.call('ZRANGE', KEYS[1], '-inf', ARGV[1], 'BYSCORE', 'LIMIT', 0, ARGV[2])
	if #jobs > 0 then
		for i, jobKey in ipairs(jobs) do
			local jobStatus = redis.call('HGET', jobKey, 'status')

			local newScore = ARGV[1] + math.random(ARGV[3])
			redis.call('ZREM', KEYS[1], jobKey)
			redis.call('DEL', 'lease:' .. jobKey)

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

// RecoverAbandonedJobs re-queues jobs that have exceeded their processing deadline.
// It uses a Lua script to atomically:
//  1. Fetch up to 'batchSize' jobs from the processing_queue whose deadline scores are less than or equal to the current time.
//  2. For each abandoned job:
//     a. Fetch its status from the job hash.
//     b. Remove it from the processing_queue and delete its expired lease.
//     c. If it wasn't cancelled, add it back to the main_queue with a new score (random backoff) and update status to 'Recovering'.
//     d. If it was cancelled, delete the job hash entirely.
func (rdb *RDB) RecoverAbandonedJobs(ctx context.Context, batchSize int) error {
	now := time.Now().Unix()

	err := recoverScript.Run(ctx, rdb.client, []string{ProcessingQueue, MainQueue}, now, batchSize, MaxRecoveryBackoff.Seconds()).Err()
	if err != nil {
		return fmt.Errorf("failed to recover jobs : %w", err)
	}

	return nil
}

// generateJobKey creates a unique Redis key for a job's hash data based on its ID.
func generateJobKey(ID string) string {
	return "job:" + ID
}

// generateLeaseKey creates a unique Redis key for a job's lease data based on its ID.
func generateLeaseKey(ID string) string {
	return "lease:job:" + ID
}

// generateLeaseID generates a new UUID v7 to be used as a unique lease identifier.
func generateLeaseID() (string, error) {
	b, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	return b.String(), nil
}
