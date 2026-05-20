package shared

import "time"

type Result struct {
	ResultID   string    `json:"result_id"`
	JobID      string    `json:"job_id"`
	FinishTime time.Time `json:"finish_time"`
	Body       any       `json:"body"`
}
