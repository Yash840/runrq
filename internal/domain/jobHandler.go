package domain

type JobHandler interface{
	Process(job Job) error
}