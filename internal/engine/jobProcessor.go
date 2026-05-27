package engine

import (
	"github.com/Yash840/runrq/internal/shared"
)

type JobProcessor interface {
	Process(job *shared.Job) (any, error)
}
