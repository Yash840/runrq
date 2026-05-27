package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/Yash840/runrq/internal/shared"
)

type JobRegistry struct {
	mp sync.Map
}

func (j *JobRegistry) GetProc(t string) (JobProcessor, error) {
	proc, ok := j.mp.Load(t)
	if !ok {
		return nil, fmt.Errorf("invalid job type: %s", t)
	}

	return proc.(JobProcessor), nil
}

func (j *JobRegistry) SetProc(t string, p JobProcessor) {
	j.mp.Store(t, p)
}

func NewDefaultJobRegistry() *JobRegistry {
	var registry = JobRegistry{mp: sync.Map{}}

	registry.SetProc("send_email", &SendEmailProcessor{})

	return &registry
}

type SendEmailProcessor struct{}

func (s *SendEmailProcessor) Process(job *shared.Job) (any, error) {
	time.Sleep(5 * time.Second)

	return nil, nil
}
