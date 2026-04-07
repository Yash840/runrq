package domain

import "log"

type JobProcessor struct {
	JobStore
	Processor func(job Job) (any, error)
}

func (j JobProcessor) Process(job Job) {
	j.MakeJobProcessing(job.ID)

	result, err := j.Processor(job)

	if err != nil {
		log.Panicf("job #%s failed with error: %s", job.ID, err.Error())
		j.MakeJobFailed(job.ID, err.Error())
		return  
	}

	j.MakeJobCompleted(job.ID, result)
}
