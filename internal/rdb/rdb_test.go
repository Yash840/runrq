package rdb

import (
	"context"
	"log"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/Yash840/runrq/internal/shared"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

var (
	rdb shared.Broker
	mr  *miniredis.Miniredis
)

func TestMain(m *testing.M) {
	code := setupAndRun(m)
	os.Exit(code)
}

func setupAndRun(m *testing.M) int {
	var err error
	mr, err = miniredis.Run()
	if err != nil {
		log.Fatalf("failed to initialize Miniredis instance: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func(client *redis.Client) {
		err := client.Close()
		if err != nil {
			log.Panicf("failed to gracefully close Redis client: %v", err)
		}
	}(client)

	rdb = &RDB{
		client: client,
	}

	return m.Run()
}

func TestRDB_Submit(t *testing.T) {
	dummyJobID := uuid.New().String()
	dummyJob := &shared.Job{
		JobID:        dummyJobID,
		OwnerID:      "test-owner-123",
		JobType:      "email_processing",
		Payload:      map[string]interface{}{"to": "test@example.com"},
		Status:       shared.JobStatusQueued,
		ScheduleTime: time.Now(),
		NextRunAt:    time.Now().Add(5 * time.Minute),
		RetryPolicy:  shared.RetryPolicyModerate,
		MaxRetries:   3,
		RetriesDone:  0,
		StoreResult:  true,
		MaxTimeout:   60,
	}

	ctx := context.Background()
	err := rdb.Submit(ctx, dummyJob)
	if err != nil {
		t.Fatalf("failed to submit test job: %v", err)
	}

	var submittedJobScore float64
	submittedJobScore, err = mr.ZScore(MainQueue, generateJobKey(dummyJobID))
	assert.Equal(t, submittedJobScore, float64(dummyJob.NextRunAt.Unix()), "The job's score in the main queue ZSET must equal its NextRunAt Unix timestamp")

	jobKey := "job:" + dummyJobID
	status := mr.HGet(jobKey, "status")

	if status != "Queued" {
		t.Errorf("Expected job status to be 'Queued', but got '%s'", status)
	}
}

func TestRDB_Cancel(t *testing.T) {
	ctx := context.Background()

	t.Run("Cancel exactly from main_queue", func(t *testing.T) {
		jobID := uuid.New().String()
		jobKey := generateJobKey(jobID)

		_, err := mr.ZAdd(MainQueue, 100, jobKey)
		if err != nil {
			t.Fatalf("Miniredis command failed: %v", err)
		}
		mr.HSet(jobKey, "status", "Queued")

		err = rdb.Cancel(ctx, jobID)
		if err != nil {
			t.Fatalf("failed to cancel job: %v", err)
		}

		mq, _ := mr.ZMembers(MainQueue)
		isJobPresent := slices.Contains(mq, jobKey)
		assert.False(t, isJobPresent, "The job key must be removed from the main queue following cancellation")

		mr.Del(jobKey)
		isHashPresent := mr.Exists(jobKey)
		assert.False(t, isHashPresent, "The job hash must be deleted following cancellation")
	})

	t.Run("Cancel exactly from ready_queue", func(t *testing.T) {
		jobID := uuid.New().String()
		jobKey := generateJobKey(jobID)

		_, err := mr.Lpush(ReadyQueue, jobKey)
		if err != nil {
			t.Fatalf("Miniredis command failed: %v", err)
		}
		mr.HSet(jobKey, "status", "Queued")

		err = rdb.Cancel(ctx, jobID)
		if err != nil {
			t.Fatalf("failed to cancel job: %v", err)
		}

		rq, _ := mr.List(ReadyQueue)
		isJobInReadyQ := slices.Contains(rq, jobKey)
		assert.False(t, isJobInReadyQ, "The job key must be removed from the ready queue following cancellation")

		mr.Del(jobKey)
		isHashPresent := mr.Exists(jobKey)
		assert.False(t, isHashPresent, "The job hash must be deleted following cancellation")
	})

	t.Run("Cancel exactly from processing_queue", func(t *testing.T) {
		jobID := uuid.New().String()
		jobKey := generateJobKey(jobID)

		_, err := mr.ZAdd(ProcessingQueue, 100, jobKey)
		if err != nil {
			t.Fatalf("Miniredis command failed: %v", err)
		}
		mr.HSet(jobKey, "status", "Processing")

		err = rdb.Cancel(ctx, jobID)
		if err != nil {
			t.Fatalf("failed to cancel job: %v", err)
		}

		jobStatus := mr.HGet(jobKey, "status")
		assert.Equal(t, jobStatus, "Cancelled", "The job status must be updated to 'Cancelled' following the cancellation of an in-process job")
	})
}

func TestRDB_Acquire(t *testing.T) {
	ctx := context.Background()

	t.Run("Acquire when no jobs in ready_queue", func(t *testing.T) {
		_, _, err := rdb.Acquire(ctx)
		assert.Error(t, err, "An error should be returned when acquiring from an empty ready queue")
	})

	t.Run("Acquire when there are jobs in ready_queue", func(t *testing.T) {
		dummyJobID := uuid.New().String()
		dummyJob := &shared.Job{
			JobID:        dummyJobID,
			OwnerID:      "test-owner-123",
			JobType:      "email_processing",
			Payload:      map[string]interface{}{"to": "test@example.com"},
			Status:       shared.JobStatusQueued,
			ScheduleTime: time.Now(),
			NextRunAt:    time.Now().Add(5 * time.Minute),
			RetryPolicy:  shared.RetryPolicyModerate,
			MaxRetries:   3,
			RetriesDone:  0,
			StoreResult:  true,
			MaxTimeout:   60,
		}

		mr.HSet(generateJobKey(dummyJobID),
			"job_id", dummyJobID,
			"owner_id", dummyJob.OwnerID,
			"job_type", dummyJob.JobType,
			"payload", `{"to":"test@example.com"}`,
			"status", "Queued",
			"schedule_time", dummyJob.ScheduleTime.Format(time.RFC3339),
			"next_run_at", dummyJob.NextRunAt.Format(time.RFC3339),
			"retry_policy", dummyJob.RetryPolicy.String(),
			"max_retries", "3",
			"retries_done", "0",
			"store_result", "true",
			"max_timeout", "60",
		)

		_, err := mr.Lpush(ReadyQueue, generateJobKey(dummyJobID))
		if err != nil {
			t.Fatalf("Miniredis command failed: %v", err)
		}

		var leaseId string
		var job *shared.Job
		job, leaseId, err = rdb.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}

		assert.NotEmpty(t, leaseId, "The returned lease ID must not be empty")
		assert.Equal(t, dummyJob.JobID, job.JobID, "The acquired job ID must match the inputted dummy job ID")

		jobStatus := mr.HGet(generateJobKey(dummyJobID), "status")
		assert.Equal(t, jobStatus, shared.JobStatusProcessing.String(), "The job status must be transitioned to 'Processing' upon acquisition")

		isLeaseAdded := mr.HGet(generateJobKey(dummyJobID), "lease_id") != ""
		assert.True(t, isLeaseAdded, "A lease record must be created for the acquired job")
	})
}

func TestRDB_MarksAsCompleted(t *testing.T) {
	ctx := context.Background()

	jobId := uuid.New().String()
	leaseID := uuid.New().String()

	_, err := mr.ZAdd(ProcessingQueue, 100, generateJobKey(jobId))
	if err != nil {
		t.Fatalf("Miniredis command failed: %v", err)
	}
	mr.HSet(generateJobKey(jobId), "status", "Processing", "lease_id", leaseID)

	var pq []string

	t.Run("attempting to mark job completed with incorrect lease", func(t *testing.T) {
		err := rdb.MarksAsCompleted(ctx, jobId, "incorrectLease1234")
		assert.Error(t, err, "An error must be returned when providing an incorrect lease ID")

		pq, err = mr.ZMembers(ProcessingQueue)
		if err != nil {
			t.Fatalf("Miniredis command failed: %v", err)
		}

		isJobInPq := slices.Contains(pq, generateJobKey(jobId))
		assert.True(t, isJobInPq, "The job must remain in the processing queue when completion is attempted with an invalid lease")

		isThereJobHash := mr.Exists(generateJobKey(jobId))
		assert.True(t, isThereJobHash, "The job hash must not be modified when completion is attempted with an invalid lease")

		isThereLeaseField := mr.HGet(generateJobKey(jobId), "lease_id") != ""
		assert.True(t, isThereLeaseField, "The job lease must not be modified when completion is attempted with an invalid lease")
	})

	t.Run("attempting to mark job completed with correct lease", func(t *testing.T) {
		err := rdb.MarksAsCompleted(ctx, jobId, leaseID)
		assert.NoError(t, err, "No error should be returned when marking a job as completed with a valid lease ID")

		isPqExist := mr.Exists(ProcessingQueue)

		if isPqExist {
			pq, err = mr.ZMembers(ProcessingQueue)
			if err != nil {
				t.Fatalf("Miniredis command failed: %v", err)
			}

			isJobInPq := slices.Contains(pq, generateJobKey(jobId))
			assert.False(t, isJobInPq, "The job must be removed from the processing queue upon successful completion")
		}

		isThereJobHash := mr.Exists(generateJobKey(jobId))
		assert.False(t, isThereJobHash, "The job hash must be deleted upon successful completion")

	})
}

func TestRDB_PollReadyJobs(t *testing.T) {
	ctx := context.Background()

	jobID1 := uuid.New().String()
	jobID2 := uuid.New().String()

	now := time.Now().Unix()

	//Simulating ready to execute job
	_, err := mr.ZAdd(MainQueue, float64(now), generateJobKey(jobID1))
	if err != nil {
		t.Fatalf("Miniredis command failed: %v", err)
	}

	//Simulating delayed job
	_, err = mr.ZAdd(MainQueue, float64(now+120), generateJobKey(jobID2))
	if err != nil {
		t.Fatalf("Miniredis command failed: %v", err)
	}

	err = rdb.PollReadyJobs(ctx, 10)
	if err != nil {
		t.Fatalf("Failed to poll ready jobs: %v", err)
	}

	var mq []string
	mq, err = mr.ZMembers(MainQueue)
	if err != nil {
		t.Fatalf("Miniredis command failed: %v", err)
	}

	var rq []string
	rq, err = mr.List(ReadyQueue)
	if err != nil {
		t.Fatalf("Miniredis command failed: %v", err)
	}

	isDelayedJobInMq := slices.Contains(mq, generateJobKey(jobID2))
	assert.True(t, isDelayedJobInMq, "The delayed job should remain in the main queue")

	isDelayedJobInRq := slices.Contains(rq, generateJobKey(jobID2))
	assert.False(t, isDelayedJobInRq, "The delayed job should not be transferred to the ready queue")

	isReadyJobInRq := slices.Contains(rq, generateJobKey(jobID1))
	assert.True(t, isReadyJobInRq, "The ready job must be transferred to the ready queue")

	isReadyJobInMq := slices.Contains(mq, generateJobKey(jobID1))
	assert.False(t, isReadyJobInMq, "The ready job must be removed from the main queue")
}

func TestRDB_ExtendDeadline(t *testing.T) {
	ctx := context.Background()

	jobId := uuid.New().String()
	leaseID := uuid.New().String()

	now := time.Now().Unix()

	mr.HSet(generateJobKey(jobId), "lease_id", leaseID)
	_, err := mr.ZAdd(ProcessingQueue, float64(now), generateJobKey(jobId))
	if err != nil {
		t.Fatalf("Miniredis command failed: %v", err)
	}

	t.Run("lease extension with wrong lease", func(t *testing.T) {
		err := rdb.ExtendDeadline(ctx, jobId, "wrongLease1234", 30*time.Second)
		assert.Error(t, err, "An error must be returned when extending a deadline with an incorrect lease ID")
	})

	t.Run("lease extension with correct lease", func(t *testing.T) {
		err := rdb.ExtendDeadline(ctx, jobId, leaseID, 30*time.Second)
		assert.NoError(t, err, "No error should be returned when extending a deadline with a valid lease ID")

		updatedDeadline, err := mr.ZScore(ProcessingQueue, generateJobKey(jobId))
		if err != nil {
			t.Fatalf("Miniredis command failed: %v", err)
		}

		expectedDeadline := float64(now + 30)
		assert.Equal(t, expectedDeadline, updatedDeadline, "The updated deadline in the processing queue must match the expected new deadline")
	})
}

func TestRDB_ResubmitJob(t *testing.T) {
	ctx := context.Background()

	jobID := uuid.New().String()
	jobKey := generateJobKey(jobID)

	now := time.Now().Unix()

	_, err := mr.ZAdd(ProcessingQueue, float64(now+30), jobKey)
	if err != nil {
		t.Fatalf("Miniredis command failed: %v", err)
	}

	mr.HSet(jobKey, "status", "Processing", "lease_id", "some-lease-id")

	nextRunAfter := 5 * time.Minute
	err = rdb.ResubmitJob(ctx, nextRunAfter, &shared.Job{
		JobID:       jobID,
		RetriesDone: 1,
		NextRunAt:   time.Unix(now+int64(nextRunAfter.Seconds()), 0),
	})
	assert.NoError(t, err, "No error should be returned when resubmitting a job")

	pq, _ := mr.ZMembers(ProcessingQueue)
	isJobInPq := slices.Contains(pq, jobKey)
	assert.False(t, isJobInPq, "The job must be removed from the processing queue")

	mq, _ := mr.ZMembers(MainQueue)
	isJobInMq := slices.Contains(mq, jobKey)
	assert.True(t, isJobInMq, "The job must be added back to the main queue")

	status := mr.HGet(jobKey, "status")
	assert.Equal(t, status, "Retrying", "The job status must be updated to 'Retrying'")

	isLeaseExist := mr.HGet(jobKey, "lease_id") != ""
	assert.False(t, isLeaseExist, "The lease must be cleared")

	score, err := mr.ZScore(MainQueue, jobKey)
	if err != nil {
		t.Fatalf("Failed to fetch score from main queue: %v", err)
	}
	expectedScore := float64(time.Now().Unix() + int64(nextRunAfter.Seconds()))
	assert.InDelta(t, expectedScore, score, 2.0, "The score in main queue must match the calculated nextRunAt")
}
