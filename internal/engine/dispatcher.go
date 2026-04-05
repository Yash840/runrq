package engine

import (
	"fmt"
	"sync"
	"github.com/Yash840/runrq/internal/domain"
)

type Dispatcher struct {
	jobQueue 	chan domain.Job
	maxWorkers 	int
	wg 			*sync.WaitGroup
	domain.JhRegistry
}

func NewDispatcher(maxWorkers int, maxQueueSize int, registry domain.JhRegistry) *Dispatcher {
	return &Dispatcher{
		jobQueue: make(chan domain.Job, maxQueueSize),
		maxWorkers: maxWorkers,
		wg: new(sync.WaitGroup),
		JhRegistry: registry,
	}
}

func (d Dispatcher) worker(id int) {
	defer d.wg.Done()

	for job := range d.jobQueue {
		handler, _ := d.GetHandler(job.Type)
		fmt.Printf("worker #%d processing job #%d\n", id, job.ID)
		handler.Process(job)
	}
}

func (d Dispatcher) Start() {
	for i := 1; i <= d.maxWorkers; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}
}

func (d Dispatcher) Stop() {
	close(d.jobQueue)
	d.wg.Wait()
}

func (d Dispatcher) Submit(job domain.Job) {
	d.jobQueue <- job
}