package rdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Yash840/runrq/internal/shared"
	"github.com/redis/go-redis/v9"
)

type RDB struct {
	client *redis.Client
}

func (rdb *RDB) Submit(ctx context.Context, job *shared.Job) error {
	nextRunAt := job.NextRunAt.Unix()

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to submit job : %w", err)
	}

	err = rdb.client.ZAdd(ctx, "main_queue", redis.Z{
		Score:  float64(nextRunAt),
		Member: jobJSON,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to submit job : %w", err)
	}

	return nil
}

func (rdb *RDB) Cancel(ctx context.Context, jobID string) error {
	err := rdb.client.SAdd(ctx, "cancelled_queue", jobID).Err()
	if err != nil {
		return fmt.Errorf("failed to cancel job : %w", err)
	}

	return nil
}

func (rdb *RDB) IsCancelled(ctx context.Context, jobID string) (bool, error) {
	isCancelled, err := rdb.client.SIsMember(ctx, "cancelled_queue", jobID).Result()
	if err != nil {
		return false, fmt.Errorf("failed to fetch job : %w", err)
	}

	return isCancelled, nil
}

var transferScript = redis.NewScript(`
	local jobs = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
	if #jobs > 0 then 
		redis.call('ZREM', KEYS[1], unpack(jobs))
		redis.call('LPUSH', KEYS[2], unpack(jobs))

		return jobs
	end

	return {}
	`)

func (rdb *RDB) PollReadyJobs(ctx context.Context, batchSize int) error {
	now := time.Now().Unix()
	err := transferScript.Run(ctx, rdb.client, []string{"main_queue", "ready_queue"}, now, batchSize).Err()
	if err != nil {
		return fmt.Errorf("failed to poll jobs : %w", err)
	}

	return nil
}
