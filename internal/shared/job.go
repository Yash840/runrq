package shared

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	ACTION_GET    = "GET"
	ACTION_POST   = "POST"
	ACTION_DELETE = "DELETE"
	ACTION_PATCH  = "PATCH"

	STATUS_QUEUED     = "QUEUED"
	STATUS_PROCESSING = "PROCESSING"
	STATUS_FINISHED   = "FINISHED"
	STATUS_FAILED     = "FAILED"
)

type Job struct {
	JobID        string `json:"job_id"`
	UserID       string `json:"user_id"`
	Target       string
	Action       string
	Body         []byte
	AuthType     string `json:"auth_type"`
	authToken    string
	JobType      string `json:"job_type"`
	Status       string
	ScheduleTime time.Time `json:"schedule_time"`
	NextRunAt    time.Time `json:"next_run_at"`
	MaxRetries   int       `json:"max_retries"`
	RetriesDone  int       `json:"retries_done"`
}

type JobOpts struct {
	userID       string
	target       string
	action       string
	body         []byte
	authType     string
	authToken    string
	jobType      string
	status       string
	scheduleTime time.Time
	maxRetries   int
}

func NewJob(opts JobOpts) (Job, error) {
	if opts.userID == "" {
		return Job{}, errors.New("userID is required")
	}
	if opts.target == "" {
		return Job{}, errors.New("target is required")
	}

	switch opts.action {
	case "":
		opts.action = ACTION_GET
	case ACTION_GET, ACTION_POST, ACTION_DELETE, ACTION_PATCH:
	default:
		return Job{}, errors.New("invalid action")
	}

	if opts.scheduleTime.IsZero() {
		opts.scheduleTime = time.Now()
	}

	if opts.maxRetries < 0 {
		return Job{}, errors.New("maxRetries cannot be negative")
	}

	jobID, err := newJobID()
	if err != nil {
		return Job{}, err
	}

	return Job{
		JobID:        jobID,
		UserID:       opts.userID,
		Target:       opts.target,
		Action:       opts.action,
		Body:         opts.body,
		AuthType:     opts.authType,
		authToken:    opts.authToken,
		JobType:      opts.jobType,
		Status:       STATUS_QUEUED,
		ScheduleTime: opts.scheduleTime,
		NextRunAt:    opts.scheduleTime,
		MaxRetries:   opts.maxRetries,
		RetriesDone:  0,
	}, nil
}

func newJobID() (string, error) {
	b, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	return b.String(), nil
}
