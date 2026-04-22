package engine

import (
	"log"

	"github.com/Yash840/runrq/internal/domain"
	"github.com/Yash840/runrq/internal/repository"
)

type JobProcessor struct {
	repository.JobRecordsRepo
	Processor func(job domain.Job) (any, error)
}

func (j JobProcessor) Process(job domain.Job) {
	j.JobRecordsRepo.MakeJobProcessing(job.ID)

	result, err := j.Processor(job)

	if err != nil {
		log.Panicf("job #%s failed with error: %s", job.ID, err.Error())
		j.JobRecordsRepo.MakeJobFailed(job.ID, err.Error())
		return
	}

	j.JobRecordsRepo.MakeJobCompleted(job.ID, result)
}
