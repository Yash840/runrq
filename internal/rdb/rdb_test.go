package rdb

import (
	"context"
	"encoding/json"
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
	var err error
	mr, err = miniredis.Run()
	if err != nil {
		log.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func(client *redis.Client) {
		err := client.Close()
		if err != nil {
			log.Panic("failed to close redis client")
		}
	}(client)

	rdb = &RDB{
		client: client,
	}

	code := m.Run()

	os.Exit(code)
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
		t.Fatalf("failed to submit job: %v", err)
	}

	var submittedJobScore float64
	submittedJobScore, err = mr.ZScore(MainQueue, generateJobKey(dummyJobID))
	assert.Equal(t, submittedJobScore, float64(dummyJob.NextRunAt.Unix()), "job's score in ZSET must be same as NextRunAt")

	jobKey := "job:" + dummyJobID
	status := mr.HGet(jobKey, "status")

	if status != "Queued" {
		t.Errorf("Expected status to be 'Queued', got %s", status)
	}
}

func TestRDB_Cancel(t *testing.T) {
	ctx := context.Background()

	t.Run("Cancel exactly from main_queue", func(t *testing.T) {
		jobID := uuid.New().String()
		jobKey := generateJobKey(jobID)

		_, err := mr.ZAdd(MainQueue, 100, jobKey)
		if err != nil {
			t.Fatalf("miniredis failure: %v", err)
		}
		mr.HSet(jobKey, "status", "Queued")

		err = rdb.Cancel(ctx, jobID)
		if err != nil {
			t.Fatalf("failed to cancel job: %v", err)
		}

		mq, _ := mr.ZMembers(MainQueue)
		isJobPresent := slices.Contains(mq, jobKey)
		assert.False(t, isJobPresent, "After cancellation, job key must deleted from main queue")

		mr.Del(jobKey)
		isHashPresent := mr.Exists(jobKey)
		assert.False(t, isHashPresent, "After cancellation, job hash must deleted")
	})

	t.Run("Cancel exactly from ready_queue", func(t *testing.T) {
		jobID := uuid.New().String()
		jobKey := generateJobKey(jobID)

		_, err := mr.Lpush(ReadyQueue, jobKey)
		if err != nil {
			t.Fatalf("miniredis failure: %v", err)
		}
		mr.HSet(jobKey, "status", "Queued")

		err = rdb.Cancel(ctx, jobID)
		if err != nil {
			t.Fatalf("failed to cancel job: %v", err)
		}

		rq, _ := mr.List(ReadyQueue)
		isJobInReadyQ := slices.Contains(rq, jobKey)
		assert.False(t, isJobInReadyQ, "After cancellation, job key must deleted from ready queue")

		mr.Del(jobKey)
		isHashPresent := mr.Exists(jobKey)
		assert.False(t, isHashPresent, "After cancellation, job hash must deleted")
	})

	t.Run("Cancel exactly from processing_queue", func(t *testing.T) {
		jobID := uuid.New().String()
		jobKey := generateJobKey(jobID)

		_, err := mr.ZAdd(ProcessingQueue, 100, jobKey)
		if err != nil {
			t.Fatalf("miniredis failure: %v", err)
		}
		mr.HSet(jobKey, "status", "Processing")

		err = rdb.Cancel(ctx, jobID)
		if err != nil {
			t.Fatalf("failed to cancel job: %v", err)
		}

		jobStatus := mr.HGet(jobKey, "status")
		assert.Equal(t, jobStatus, "Cancelled", "After In-Process job cancellation, Job Status must be 'Cancelled'")
	})
}

func TestRDB_Acquire(t *testing.T) {
	ctx := context.Background()

	t.Run("Acquire when no jobs in ready_queue", func(t *testing.T) {
		_, _, err := rdb.Acquire(ctx)
		assert.Error(t, err, "when there are no jobs error must be thrown")
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

		jobJSON, err := json.Marshal(dummyJob)
		if err != nil {
			t.Fatalf("failed to marshal job into json: %v", err)
		}

		_, err = mr.Lpush(ReadyQueue, generateJobKey(dummyJobID))
		if err != nil {
			t.Fatalf("miniredis failure: %v", err)
		}

		mr.HSet(generateJobKey(dummyJobID), "status", "Queued", "job-json", string(jobJSON))

		var leaseId string
		var job *shared.Job
		job, leaseId, err = rdb.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}

		assert.NotEmpty(t, leaseId, "leaseID must not empty")
		assert.Equal(t, dummyJob.JobID, job.JobID, "returned job ID must be the same as the inputted job ID")

		jobStatus := mr.HGet(generateJobKey(dummyJobID), "status")
		assert.Equal(t, jobStatus, shared.JobStatusProcessing.String(), "after acquisition job status must be updated to processing")

		isLeaseAdded := mr.Exists(generateLeaseKey(dummyJobID))
		assert.True(t, isLeaseAdded, "after acquisition job lease must be added")
	})
}
