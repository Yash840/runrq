package internal

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/Yash840/runrq/internal/engine"
	"github.com/Yash840/runrq/internal/rdb"
	"github.com/Yash840/runrq/internal/shared"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	d *engine.Dispatcher
	b shared.Broker

	cancellations *sync.Map

	ctx context.Context
}

func NewClient(db *sql.DB, rc *redis.Client, concurrency int) *Client {
	c := &sync.Map{}

	d := engine.NewDispatcher(concurrency, db, rc, c)
	b := rdb.NewRDB(rc)

	ctx := context.Background()

	return &Client{d: d, b: b, cancellations: c, ctx: ctx}
}

func NewListeningClient(db *sql.DB, rc *redis.Client, concurrency int) *Client {
	client := NewClient(db, rc, concurrency)
	client.Listen()

	return client
}

func (c Client) Listen() {
	c.d.Start()
	fmt.Println("runrq client is listening")
}

func (c Client) Close() {
	c.ctx.Done()
	c.d.Shutdown()
}

func (c Client) Submit(opts shared.JobOpts) (string, error) {
	job, err := shared.NewJob(opts)
	if err != nil {
		return "", fmt.Errorf("error in creating new job: %w", err)
	}

	err = c.b.Submit(c.ctx, &job)
	if err != nil {
		return "", fmt.Errorf("error in submitting job: %w", err)
	}

	return job.JobID, nil
}

func (c Client) Cancel(jobID string) error {
	err := c.b.Cancel(c.ctx, jobID)
	if err != nil {
		return fmt.Errorf("error in cancelling job: %w", err)
	}

	cancel, ok := c.cancellations.LoadAndDelete(jobID)
	if ok {
		cancel.(context.CancelFunc)()
	}

	return nil
}
