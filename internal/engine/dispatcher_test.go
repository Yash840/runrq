package engine

import (
	"testing"
	"time"
	"github.com/Yash840/runrq/internal/domain"
)

type EmailHandler struct{}

func (EmailHandler) Process(job domain.Job) error {
	time.Sleep(2 * time.Second)
	return nil
}

func BenchmarkDispatcher(b *testing.B) {
	handlerMap := map[string]domain.JobHandler{"email": EmailHandler{}}
	registry := domain.DefaultRegistry{Registry: handlerMap}

	disp := NewDispatcher(10, 100, registry)
	disp.Start()

	for i := 1; i < 1001; i++ {
		disp.Submit(domain.Job{
			ID: i,
			Type: "email",
			Payload: make([]byte, 10),
		})
	}
}