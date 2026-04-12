package model

import "time"

type JobRecordUpdateOpts struct {
	Status      *string
	Result      any
	Error       *string
	CompletedAt *time.Time
}