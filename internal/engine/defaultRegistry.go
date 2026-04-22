package engine

import (
	"errors"
	"time"

	"github.com/Yash840/runrq/internal/domain"
)

type DefaultRegistry struct {
	Registry map[string]domain.JobHandler
}

var EmailHandler JobProcessor = JobProcessor{Processor: EmailProcessor, JobRecordsRepo: *GetJobStoreInstance()}

func EmailProcessor(job domain.Job) (any, error) {
	time.Sleep(10 * time.Second)
	return nil, nil
}

func NewDefaultRegistry() *DefaultRegistry {
	handlerMap := map[string]domain.JobHandler{"email": EmailHandler}
	return &DefaultRegistry{Registry: handlerMap}
}

func (dr DefaultRegistry) GetHandler(Type string) (domain.JobHandler, error) {
	handler, ok := dr.Registry[Type]

	if ok == false {
		return handler, errors.New("handler for given type is not available")
	}

	return handler, nil
}

func (dr DefaultRegistry) AddHandler(Type string, Handler domain.JobHandler) {
	dr.Registry[Type] = Handler
}
