package shared

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	JobID   string    `json:"job_id" redis:"job_id"`
	OwnerID string    `json:"owner_id" redis:"owner_id"`
	JobType string    `json:"job_type" redis:"job_type"`
	Payload any       `json:"payload" redis:"payload"`
	Status  JobStatus `json:"status" redis:"status"`

	//Time at which Job is intended to be executed
	ScheduleTime time.Time `json:"schedule_time" redis:"schedule_time"`

	//Time at which Job will be executed next
	NextRunAt time.Time `json:"next_run_at" redis:"next_run_at"`

	RetryPolicy RetryPolicy `json:"retry_policy" redis:"retry_policy"`
	MaxRetries  int         `json:"max_retries" redis:"max_retries"`
	RetriesDone int         `json:"retries_done" redis:"retries_done"`

	//Boolean to tell engine, should store the result or ignore it
	StoreResult bool `json:"store_result" redis:"store_result"`

	MaxTimeout int `json:"max_timeout" redis:"max_timeout"`
}

type JobOpts struct {
	ownerID      string
	payload      any
	jobType      string
	status       JobStatus
	scheduleTime time.Time
	RetryPolicy  RetryPolicy
	maxRetries   int
	StoreResult  bool
	MaxTimeout   int
}

func NewJob(opts JobOpts) (Job, error) {
	if opts.ownerID == "" {
		return Job{}, errors.New("ownerID is required")
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
		OwnerID:      opts.ownerID,
		Payload:      opts.payload,
		JobType:      opts.jobType,
		Status:       JobStatusQueued,
		ScheduleTime: opts.scheduleTime,
		NextRunAt:    opts.scheduleTime,
		RetryPolicy:  opts.RetryPolicy,
		MaxRetries:   opts.maxRetries,
		RetriesDone:  0,
		StoreResult:  opts.StoreResult,
		MaxTimeout:   opts.MaxTimeout,
	}, nil
}

func newJobID() (string, error) {
	b, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	return b.String(), nil
}

type JobStatus int

const (
	JobStatusQueued JobStatus = iota + 1
	JobStatusProcessing
	JobStatusRetrying
	JobStatusSucceed
	JobStatusQueuedForNext
	JobStatusFailed
)

func (j JobStatus) String() string {
	switch j {
	case JobStatusQueued:
		return "Queued"
	case JobStatusProcessing:
		return "Processing"
	case JobStatusRetrying:
		return "Retrying"
	case JobStatusSucceed:
		return "Succeed"
	case JobStatusQueuedForNext:
		return "Queued-For-Next"
	case JobStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

func (j JobStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.String())
}

func (j *JobStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case "Queued":
		*j = JobStatusQueued
	case "Processing":
		*j = JobStatusProcessing
	case "Retrying":
		*j = JobStatusRetrying
	case "Succeed":
		*j = JobStatusSucceed
	case "Queued-For-Next":
		*j = JobStatusQueuedForNext
	case "Failed":
		*j = JobStatusFailed
	default:
		return errors.New("invalid JobState")
	}
	return nil
}

func (j JobStatus) MarshalBinary() ([]byte, error) {
	return []byte(j.String()), nil
}

func (j *JobStatus) UnmarshalBinary(data []byte) error {
	str := string(data)
	switch str {
	case "Queued":
		*j = JobStatusQueued
	case "Processing":
		*j = JobStatusProcessing
	case "Retrying":
		*j = JobStatusRetrying
	case "Succeed":
		*j = JobStatusSucceed
	case "Queued-For-Next":
		*j = JobStatusQueuedForNext
	case "Failed":
		*j = JobStatusFailed
	default:
		return errors.New("invalid JobState")
	}
	return nil
}

type RetryPolicy int

const (
	RetryPolicyImmediate RetryPolicy = iota + 1
	RetryPolicyModerate
	RetryPolicyDelayed
	RetryPolicyNone
)

func (r RetryPolicy) String() string {
	switch r {
	case RetryPolicyImmediate:
		return "Immediate"
	case RetryPolicyModerate:
		return "Moderate"
	case RetryPolicyDelayed:
		return "Delayed"
	default:
		return "None"
	}
}

func (r RetryPolicy) MarshalBinary() ([]byte, error) {
	return []byte(r.String()), nil
}

func (r *RetryPolicy) UnmarshalBinary(data []byte) error {
	str := string(data)
	switch str {
	case "Immediate":
		*r = RetryPolicyImmediate
	case "Moderate":
		*r = RetryPolicyModerate
	case "Delayed":
		*r = RetryPolicyDelayed
	case "None":
		*r = RetryPolicyNone
	default:
		return errors.New("invalid RetryPolicy")
	}
	return nil
}
