package domain

import (
	"errors"
	"time"
)

type DefaultRegistry struct {
	Registry map[string]JobProcessor
}

var EmailHandler JobProcessor = JobProcessor{Processor: EmailProcessor, JobStore: *GetJobStoreInstance()}

func EmailProcessor(job Job) (any, error) {
	time.Sleep(10 * time.Second)
	return nil, nil
}

func NewDefaultRegistry() *DefaultRegistry {
	handlerMap := map[string]JobProcessor{"email": EmailHandler}
	return &DefaultRegistry{Registry: handlerMap}
}

func (dr DefaultRegistry) GetHandler(Type string) (JobProcessor, error) {
	handler, ok := dr.Registry[Type]

	if ok == false {
		return handler, errors.New("handler for given type is not available")
	}

	return handler, nil
}

func (dr DefaultRegistry) AddHandler(Type string, Handler JobProcessor) {
	dr.Registry[Type] = Handler
}
