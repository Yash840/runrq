package engine

import (
	"fmt"
	"testing"

	"github.com/Yash840/runrq/internal/domain"
)

func BenchmarkDispatcher(b *testing.B) {
	registry := NewDefaultRegistry()

	disp := NewDispatcher(10, 100, registry)
	disp.Start()

	for i := 1; i < 1001; i++ {
		disp.Submit(domain.Job{
			ID:      fmt.Sprintf("%d", i),
			Type:    "email",
			Payload: make([]byte, 10),
		})
	}
}
