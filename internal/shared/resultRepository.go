package shared

type ResultRepository interface {
	Create(result Result) error
	Remove(resID string) error
	GetForJobID(jobID string) ([]Result, error)
	Sweep(durationHrs int) error
}
